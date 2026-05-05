package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
)

// /sys/fs/cgroup is the root of the cgroup v2 unified hierarchy
const cgroupRoot = "/sys/fs/cgroup"

func main() {
	memLimit := flag.String("mem", "", "Memory limit (e.g., 50M, 100M)")
	cpuLimit := flag.String("cpu", "", "CPU limit in percent (e.g., 20)")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		fmt.Printf("Usage: sudo go run main.go [--mem limit] [--cpu limit] run <command>\n")
		os.Exit(1)
	}

	switch args[0] {
	case "run":
		run(args[1:], *memLimit, *cpuLimit)
	case "child":
		child(args[1:])
	default:
		fmt.Printf("Unknown command: %s\n", args[0])
		os.Exit(1)
	}
}

func run(args []string, memLimit, cpuLimit string) {
	fmt.Printf("--- Parent: Creating isolated environment for %v ---\n", args)

	// Re-execute the current binary but call the 'child' command
	// This happens inside the new namespaces defined below.
	cmd := exec.Command("/proc/self/exe", append([]string{"child"}, args...)...)

	// CLONE_NEWUTS: New hostname namespace
	// CLONE_NEWPID: New PID namespace (child becomes PID 1)
	// CLONE_NEWNS:  New mount namespace
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Start the child process but don't wait yet, we need its PID for cgroups
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting child: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("--- Parent: Child started with PID %d ---\n", cmd.Process.Pid)

	// Apply Cgroup limits if requested
	if memLimit != "" || cpuLimit != "" {
		if err := applyCgroups(cmd.Process.Pid, memLimit, cpuLimit); err != nil {
			fmt.Fprintf(os.Stderr, "Cgroup error: %v\n", err)
			// We continue anyway for demonstration, but in production this is a failure
		}
	}

	// Wait for the child to finish
	if err := cmd.Wait(); err != nil {
		fmt.Fprintf(os.Stderr, "Child exited with error: %v\n", err)
	}
}

func child(args []string) {
	fmt.Printf("--- Child: Setting up isolation (PID: %d) ---\n", os.Getpid())

	// 1. Set Hostname (requires CLONE_NEWUTS)
	if err := syscall.Sethostname([]byte("container-lab")); err != nil {
		fmt.Fprintf(os.Stderr, "Error setting hostname: %v\n", err)
	}

	// 2. Mount /proc (requires CLONE_NEWNS)
	// We need this so that 'ps' and 'top' show only processes in this PID namespace.
	// MS_NOEXEC: Don't allow execution of binaries from this mount
	// MS_NOSUID: Ignore set-user-identifier or set-group-identifier bits
	// MS_NODEV:  Don't allow access to physical devices
	if err := syscall.Mount("proc", "/proc", "proc", 0, ""); err != nil {
		fmt.Fprintf(os.Stderr, "Error mounting /proc: %v\n", err)
	}

	// 3. Execute the target command
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running command: %v\n", err)
		os.Exit(1)
	}

	// Cleanup: Unmount /proc before exiting
	syscall.Unmount("/proc", 0)
}

func applyCgroups(pid int, memLimit, cpuLimit string) error {
	cgPath := filepath.Join(cgroupRoot, "systems-lab")

	// 1. Create the cgroup directory
	if err := os.MkdirAll(cgPath, 0755); err != nil {
		return fmt.Errorf("mkdir cgroup failed: %w", err)
	}

	// 2. Add the PID to the cgroup
	// Writing the PID to 'cgroup.procs' moves the process into this cgroup
	if err := os.WriteFile(filepath.Join(cgPath, "cgroup.procs"), []byte(strconv.Itoa(pid)), 0644); err != nil {
		return fmt.Errorf("add pid to cgroup failed: %w (are you sudo?)", err)
	}

	// 3. Apply Memory Limit
	if memLimit != "" {
		fmt.Printf("--- Parent: Applying Memory limit: %s ---\n", memLimit)
		// memory.max is the hard limit for memory usage in cgroup v2
		limit := parseBytes(memLimit)
		if err := os.WriteFile(filepath.Join(cgPath, "memory.max"), []byte(limit), 0644); err != nil {
			return fmt.Errorf("set memory limit failed: %w", err)
		}
	}

	// 4. Apply CPU Limit
	if cpuLimit != "" {
		fmt.Printf("--- Parent: Applying CPU limit: %s%% ---\n", cpuLimit)
		// cpu.max format: "quota period"
		// Period is usually 100,000 microseconds (100ms).
		// Quota is the amount of CPU time allowed in that period.
		pct, _ := strconv.Atoi(cpuLimit)
		quota := pct * 1000 // e.g. 20% of 100,000 is 20,000
		val := fmt.Sprintf("%d 100000", quota)
		if err := os.WriteFile(filepath.Join(cgPath, "cpu.max"), []byte(val), 0644); err != nil {
			return fmt.Errorf("set cpu limit failed: %w", err)
		}
	}

	return nil
}

// Simple parser for human-readable byte strings (e.g. 50M -> 52428800)
func parseBytes(s string) string {
	if s == "" {
		return "max"
	}
	unit := s[len(s)-1:]
	valStr := s[:len(s)-1]
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return s // fallback to raw string
	}

	switch unit {
	case "M", "m":
		return strconv.Itoa(val * 1024 * 1024)
	case "G", "g":
		return strconv.Itoa(val * 1024 * 1024 * 1024)
	case "K", "k":
		return strconv.Itoa(val * 1024)
	default:
		return s
	}
}
