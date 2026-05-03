# 03 signals

`signals` are software interrupts delivered to a process by the Linux kernel. This lab demonstrates **Process Governance**—the art of managing an application's lifecycle and its relationship with the operating system.

## What It Demonstrates

- **Graceful Shutdown**: Intercepting `SIGTERM` and `SIGINT` to ensure data integrity (`f.Sync()`) before exiting.
- **Hot Reloading**: Using `SIGHUP` to toggle application modes (e.g., Debug Mode) without a restart.
- **State Integrity**: Proving the difference between a "Dirty Death" (`SIGKILL`) and a "Clean Exit."
- **Asynchronous Flow**: Managing background workers via `context.Context` propagation.

## Manual Usage

Run from the repository root:

1. **Start the agent:**

   ```bash
   go run labs/03-signals/main.go
   ```

2. **Test Graceful Shutdown (Terminal 2):**

   ```bash
   kill -TERM <PID>  # or kill -15
   ```

3. **Test Hot-Reload (Terminal 2):**

   ```bash
   kill -HUP <PID>   # or kill -1
   ```

4. **Observe the Log Integrity:**

   ```bash
   tail -f labs/03-signals/telemetry.logso 
   ```

## 📖 Reference: The Linux Signal Dictionary

Signals are the "Morse Code" of the kernel. Each signal carries a specific semantic meaning.

### 1. The "Polite" Terminators (Catchable)

- **`SIGHUP` (1)**: Hangup. Modern convention uses this to trigger **Configuration Reloads**.
- **`SIGINT` (2)**: Interrupt. Sent when you press **`Ctrl+C`**. A request to stop.
- **`SIGTERM` (15)**: Terminate. The standard signal from **Kubernetes** or **systemd** asking your app to leave.

### 2. The "Hard" Stops (Uncatchable)

- **`SIGKILL` (9)**: The "Nuclear" option. The kernel deletes the process instantly. **Cleanup logic will not run.**
- **`SIGSTOP` (19)**: Pauses the process. It stays in RAM but gets 0% CPU.

### 3. The "Hardware/Logic" Faults

- **`SIGSEGV` (11)**: Segmentation Fault. Your app tried to access memory it doesn't own.
- **`SIGFPE` (8)**: Floating Point Exception. Usually caused by **Dividing by Zero**.
- **`SIGILL` (4)**: Illegal Instruction. The CPU doesn't understand what your code is trying to do.

### 4. Custom Channels

- **`SIGUSR1` (10) / `SIGUSR2` (12)**: Reserved for your own use. You can define what these do in your own software.

[Back to main README](../../README.md)
