# 01 procfs

`procfs` is a system utility that demonstrates how to read and parse the Linux `/proc` and `/sys` pseudo-filesystems to extract real-time system telemetry. It provides insights into CPU load, memory utilization, network throughput, and CPU power consumption directly from the kernel and hardware interfaces.

## What It Demonstrates

- Interacting with "virtual" files in the `/proc` and `/sys` directories.
- CLI argument parsing using the standard `flag` package.
- Implementation of the **Double-Sample Pattern** to calculate real-time rates (CPU %, KB/s, Watts).
- **GreenOps Observability**: Leveraging Intel RAPL (Running Average Power Limit) to monitor hardware power draw.
- Dynamic UI alignment for varying system configurations (e.g., interface names).

## Manual Usage

Run from the repository root:

```bash
# View all telemetry (Snapshot)
go run labs/01-procfs/main.go

# View only CPU cores live
go run labs/01-procfs/main.go --cores --live

# View network throughput live
go run labs/01-procfs/main.go --net --live

# View CPU power consumption live
sudo go run labs/01-procfs/main.go --power --live
```

## 📖 Reference: Key System Virtual Files
...

The `/proc` and `/sys` filesystems are windows into the kernel and hardware. Below are the most critical files for system observability:

### System-wide Health (via `/proc`)
- **`/proc/loadavg`**: System load averages for the last 1, 5, and 15 minutes. Represents the "run-queue" depth.
- **`/proc/uptime`**: Total seconds the system has been running and idle.
- **`/proc/meminfo`**: Detailed breakdown of memory usage (Total, Free, Available, Buffers, Cached).

### CPU & Performance (via `/proc`)
- **`/proc/stat`**: Cumulative counters (ticks) for CPU time spent in user, system, idle, and wait states. Used to calculate CPU usage %.
- **`/proc/interrupts`**: Tracks how hardware events (like network packets) are distributed across CPU cores.

### Networking (via `/proc/net`)
- **`/proc/net/dev`**: Receive/Transmit statistics (bytes, packets, errors) for every network interface.
- **`/proc/net/tcp` & `/proc/net/udp`**: Live tables of all open network sockets and their states.

### Power & Hardware (via `/sys`)
- **`/sys/class/powercap/intel-rapl/intel-rapl:0/energy_uj`**: Cumulative CPU package energy in microjoules. Used by tools like Kepler for GreenOps attribution.

### Storage & I/O (via `/proc`)
- **`/proc/diskstats`**: Cumulative I/O statistics for each disk device (reads, writes, time spent in I/O).
- **`/proc/mounts`**: The authoritative list of all currently mounted filesystems.

### Process Analysis (The `self` link)
- **`/proc/self/`**: A magic symlink to the directory of the process currently accessing it.
- **`/proc/self/fd/`**: List of all open file descriptors (useful for debugging "Too many open files").
- **`/proc/self/limits`**: The soft and hard resource limits applied to the process.
- **`/proc/self/cmdline`**: The full command-line arguments used to start the process.

[Back to main README](../../README.md)
