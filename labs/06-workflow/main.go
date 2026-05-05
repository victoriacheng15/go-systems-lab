package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	jobsDir    = "labs/06-workflow/jobs"
	cgroupRoot = "/sys/fs/cgroup/workflow-lab"
	memLimit   = "50M"
)

func main() {
	fmt.Printf("--- Systems Lab: Workflow Orchestrator (PID: %d) ---\n", os.Getpid())
	fmt.Printf("Watching: %s\n", jobsDir)
	fmt.Println("----------------------------------------------------------")

	// 1. Prepare Environment
	os.MkdirAll(jobsDir, 0755)
	os.MkdirAll(cgroupRoot, 0755)

	// 2. Setup Signal Handling for Graceful Shutdown
	ctx, cancel := context.WithCancel(context.Background())
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs
		fmt.Println("\n>> Initiating Orchestrator Shutdown...")
		cancel()
	}()

	// 3. Start the inotify Watcher
	if err := watchJobs(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Watcher error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(">> Orchestrator exited.")
}

func watchJobs(ctx context.Context) error {
	fd, err := syscall.InotifyInit()
	if err != nil {
		return err
	}
	defer syscall.Close(fd)

	_, err = syscall.InotifyAddWatch(fd, jobsDir, syscall.IN_CREATE|syscall.IN_MOVED_TO)
	if err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		syscall.Close(fd)
	}()

	var buf [syscall.SizeofInotifyEvent * 10]byte
	for {
		n, err := syscall.Read(fd, buf[:])
		if err != nil {
			if ctx.Err() != nil {
				return nil // Normal shutdown
			}
			return err
		}

		offset := 0
		for offset < n {
			if n-offset < syscall.SizeofInotifyEvent {
				break
			}
			event := (*syscall.InotifyEvent)(unsafe.Pointer(&buf[offset]))
			if event.Len > 0 {
				nameBytes := buf[offset+syscall.SizeofInotifyEvent : offset+syscall.SizeofInotifyEvent+int(event.Len)]
				name := strings.TrimRight(string(nameBytes), "\x00")
				if !strings.HasPrefix(name, ".") {
					jobPath := filepath.Join(jobsDir, name)
					fmt.Printf("[%s] TRIGGER: New job detected: %s\n", time.Now().Format("15:04:05"), name)
					go runJob(ctx, jobPath)
				}
			}
			offset += syscall.SizeofInotifyEvent + int(event.Len)
		}
	}
}

func runJob(ctx context.Context, path string) {
	// Create a unique Cgroup for this job
	jobID := strconv.FormatInt(time.Now().UnixNano(), 10)
	cgPath := filepath.Join(cgroupRoot, jobID)
	os.MkdirAll(cgPath, 0755)
	defer os.RemoveAll(cgPath)

	// Set Memory Limit (Cgroup v2)
	limit := parseBytes(memLimit)
	os.WriteFile(filepath.Join(cgPath, "memory.max"), []byte(limit), 0644)

	// Prepare Command
	cmd := exec.Command("/bin/sh", path)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // Create a process group for easier signaling
	}

	if err := cmd.Start(); err != nil {
		fmt.Printf("   [ERROR] Failed to start job %s: %v\n", path, err)
		return
	}

	pid := cmd.Process.Pid
	fmt.Printf("   [EXEC] Job %s started (PID: %d, CG: %s)\n", path, pid, jobID)

	// Assign PID to Cgroup
	os.WriteFile(filepath.Join(cgPath, "cgroup.procs"), []byte(strconv.Itoa(pid)), 0644)

	// Start Telemetry Monitor (procfs)
	jobCtx, jobCancel := context.WithTimeout(ctx, 30*time.Second)
	defer jobCancel()

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-jobCtx.Done():
			if jobCtx.Err() == context.DeadlineExceeded {
				fmt.Printf("   [SIGNAL] Job %d timed out. Sending SIGTERM.\n", pid)
				syscall.Kill(-pid, syscall.SIGTERM) // Kill the whole process group
			}
			return
		case err := <-done:
			if err != nil {
				fmt.Printf("   [EXIT] Job %d failed: %v\n", pid, err)
			} else {
				fmt.Printf("   [EXIT] Job %d completed successfully.\n", pid)
			}
			return
		case <-ticker.C:
			// Telemetry from /proc/[pid]/stat
			usage, _ := readJobStats(pid)
			fmt.Printf("   [STATS] PID %d | %s\n", pid, usage)
		}
	}
}

func readJobStats(pid int) (string, error) {
	f, err := os.Open(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return "", err
	}
	defer f.Close()
	return parseJobStats(f)
}

func parseJobStats(r io.Reader) (string, error) {
	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		return "N/A", scanner.Err()
	}
	fields := strings.Fields(scanner.Text())
	if len(fields) < 24 {
		return "N/A", nil
	}
	// rss is the 24th field in /proc/[pid]/stat
	rssPages, _ := strconv.ParseUint(fields[23], 10, 64)
	rssMB := (rssPages * 4096) / 1024 / 1024
	return fmt.Sprintf("RSS: %d MB", rssMB), nil
}

// Reuse helper from Lab 05
func parseBytes(s string) string {
	if s == "" {
		return "max"
	}
	unit := s[len(s)-1:]
	valStr := s[:len(s)-1]
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return s
	}

	switch unit {
	case "M", "m":
		return strconv.Itoa(val * 1024 * 1024)
	case "K", "k":
		return strconv.Itoa(val * 1024)
	default:
		return s
	}
}
