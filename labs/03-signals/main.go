package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const logFile = "labs/03-signals/telemetry.log"

type config struct {
	debug bool
}

func main() {
	fmt.Printf("--- Systems Lab: Telemetry Agent (PID: %d) ---\n", os.Getpid())
	fmt.Printf("Logging to: %s\n", logFile)
	fmt.Println("----------------------------------------------------------")

	cfg := &config{debug: false}
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)

	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening log: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())

	go doWork(ctx, cfg, f)

	for {
		sig := <-sigs
		if handleSignal(sig, cfg, cancel, f) {
			break
		}
	}
	fmt.Println(">> SUCCESS: Agent exited cleanly.")
}

func handleSignal(sig os.Signal, cfg *config, cancel context.CancelFunc, f *os.File) bool {
	switch sig {
	case syscall.SIGHUP:
		cfg.debug = !cfg.debug
		fmt.Printf("\n[%s] SIGHUP: Configuration reloaded. Debug Mode: %v\n",
			time.Now().Format("15:04:05"), cfg.debug)
		return false

	case syscall.SIGINT, syscall.SIGTERM:
		fmt.Printf("\n[%s] %v: Initiating graceful shutdown...\n",
			time.Now().Format("15:04:05"), sig)
		cancel()
		cleanup(f)
		return true
	}
	return false
}

func doWork(ctx context.Context, cfg *config, f *os.File) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			load, _ := getCPULoad()
			timestamp := time.Now().Format("15:04:05")
			line := fmt.Sprintf("[%s] LOAD: %s\n", timestamp, load)
			f.WriteString(line)

			if cfg.debug {
				fmt.Printf("   [DEBUG] Logged: %s", line)
			}
		}
	}
}

func cleanup(f *os.File) {
	fmt.Println(">> [Cleanup] Finalizing disk writes...")
	f.WriteString(fmt.Sprintf("[%s] --- AGENT SHUTDOWN COMMENCING ---\n", time.Now().Format("15:04:05")))
	f.Sync()
	f.Close()
}

func getCPULoad() (string, error) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return "", err
	}
	fields := strings.Fields(string(data))
	if len(fields) >= 3 {
		return fmt.Sprintf("%s %s %s", fields[0], fields[1], fields[2]), nil
	}
	return "unknown", nil
}
