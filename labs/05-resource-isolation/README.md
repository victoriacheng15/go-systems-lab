# 05 isolation

`isolation` demonstrates the kernel primitives that power modern containerization. This lab explores **Namespaces** (for visibility isolation) and **Control Groups (Cgroups) v2** (for resource governance).

## What It Demonstrates

- **Linux Namespaces**: Using `CLONE_NEWUTS`, `CLONE_NEWPID`, and `CLONE_NEWNS` to create isolated views of the system (Hostname, Process Tree, and Mounts).
- **Control Groups (Cgroups) v2**: Implementing resource limits by interacting directly with the `/sys/fs/cgroup` unified hierarchy.
- **Process Re-execution**: The standard pattern for "bootstrapping" a containerized process in Go.
- **Mount Isolation**: Mounting a fresh `/proc` filesystem so that `top` or `ps` inside the "container" only see internal processes.

## Manual Usage

Run from the repository root:

1. **Run a command in an isolated namespace:**
   ```bash
   # This will start a shell in a new namespace with a custom hostname
   sudo go run labs/05-resource-isolation/main.go run /bin/sh
   ```

2. **Verify Isolation (Inside the new shell):**
   ```bash
   hostname        # Should show 'container-lab'
   ps aux          # Should only show PID 1 (your shell)
   ls /proc        # Should be a fresh mount
   ```

3. **Test Resource Limits (Cgroups):**
   ```bash
   # Run with a memory limit (e.g., 50MB)
   sudo go run labs/05-resource-isolation/main.go --mem 50M run /bin/sh
   ```

## 📖 Reference: The Container Building Blocks

### 1. Namespaces (What you can see)
Namespaces wrap a global system resource in an abstraction that makes it appear to the processes within the namespace that they have their own isolated instance of the resource.
- **UTS**: Isolate hostname and domain name.
- **PID**: Isolate the process ID space (your process becomes PID 1).
- **Mount (NS)**: Isolate the list of mount points.
- **Network**: Isolate network interfaces, IP addresses, and routing tables.

### 2. Cgroups (What you can use)
Cgroups (Control Groups) allow the kernel to track and limit the resource usage of a group of processes.
- **`cgroup.procs`**: The list of PIDs currently in this cgroup.
- **`memory.max`**: The hard limit for memory usage.
- **`cpu.max`**: The CPU quota (formatted as `$PERIOD $QUOTA`).

### 3. The "Double Exec" Pattern
To create a namespace in Go, we set the `Cloneflags` in `syscall.SysProcAttr`. However, many initializations (like mounting `/proc` or setting the hostname) must happen *after* the namespace is created but *before* the target application starts. We achieve this by having the parent re-execute the same binary with a special internal command (`child`).

[Back to main README](../../README.md)
