# Go Systems Lab

Go Systems Lab is a dedicated sandbox for learning Linux systems programming through small, focused Go projects. Each module targets a practical kernel or system-level concept, starting with system observation and moving through event-driven monitoring and process governance.

The goal is to understand how real systems are shaped at a smaller scale: how the kernel exposes state, how processes react to filesystem events, how signal-based communication works, and how Go's concurrency model interacts with Linux primitives.

## Repository Structure

All projects are located in the `labs/` directory. Each module focuses on a specific pillar of the Linux Mastery roadmap.

- **[`01-procfs`](labs/01-procfs/README.md)**: System observation via the `/proc` pseudo-filesystem and telemetry parsing.
- **[`02-inotify`](labs/02-inotify/README.md)**: Event-driven reaction using the Linux `inotify` subsystem for real-time monitoring.
- **[`03-signals`](labs/03-signals/README.md)**: Process governance, lifecycle management, and graceful shutdown implementation.
- **[`04-epoll`](labs/04-epoll/README.md)**: High-performance network orchestration and stateful event loops.
- **[`05-resource-isolation`](labs/05-resource-isolation/README.md)**: Kernel-level isolation via Namespaces and Cgroups v2 governance.
- **[`06-workflow`](labs/06-workflow/README.md)**: The Systems Capstone, an event-driven orchestrator integrating inotify, cgroups, and telemetry.
- **[`07-seccomp-capabilities`](labs/07-seccomp-capabilities/README.md)**: Syscall-boundary governance with `seccomp` filters and Linux capability inspection.
- **[`08-netlink-route-monitoring`](labs/08-netlink-route-monitoring/README.md)**: Kernel networking event monitoring with `AF_NETLINK` and `NETLINK_ROUTE`.
- **[`09-io-uring-async-io`](labs/09-io-uring-async-io/README.md)**: Asynchronous I/O with `io_uring` submission and completion queues.
- **[`10-shared-memory-mmap`](labs/10-shared-memory-mmap/README.md)**: Shared memory IPC with file-backed `mmap` and `MAP_SHARED`.
- **[`11-ebpf-xdp`](labs/11-ebpf-xdp/README.md)**: Programmable kernel packet handling with eBPF and XDP.

## How To Use This Repo

Start with the lab directory for the project you want to inspect. Each one contains a `main.go` that demonstrates a specific concept and can be run from the repository root.

The module can also be checked as a whole:

```bash
go mod tidy
go test ./...
```
