# 02 inotify

`inotify` is a Linux kernel subsystem that extends filesystems to notice changes and report those changes to applications. This lab demonstrates how to move from inefficient **Polling** (Lab 01) to **Event-Driven Reaction**.

## What It Demonstrates

- **Kernel Notifications**: Using raw syscalls (`inotify_init`, `inotify_add_watch`) to register interest in filesystem events.
- **Blocking I/O**: Understanding how the application sleeps efficiently until the kernel "wakes it up" with data.
- **Full File Lifecycle**: Capturing `OPEN`, `ACCESS` (Read), and `CLOSE` events to see how standard utilities like `cat` or `tail` interact with the VFS layer.
- **Log Rotation Handling**: Detecting when a file is moved or deleted via `IN_MOVED_FROM` and `IN_MOVED_TO`.

## Manual Usage

Run from the repository root:

1. **Start the watcher:**

   ```bash
   # Watch a specific file to see its lifecycle
   go run labs/02-inotify/main.go labs/02-inotify/README.md
   ```

2. **Generate events (in another terminal):**

   ```bash
   cat labs/02-inotify/README.md  # Triggers OPEN -> ACCESS -> CLOSE_READ
   touch labs/02-inotify/README.md # Triggers ATTRIB
   ```

## 📖 Reference: The inotify Lifecycle

### 1. Registration

The application calls `inotify_add_watch` with an **Event Mask**. This is a bitmask that tells the kernel exactly which syscalls on that file descriptor should trigger a notification.

### 2. The Blocking Read

When the application calls `read(fd, ...)`, the kernel checks the event queue for that inotify instance. If empty, the process is moved to a **Wait Queue** and consumes 0% CPU.

### 3. Wake-up

As soon as another process (like `cat`) triggers a syscall matching your mask, the kernel:

1. Writes an `inotify_event` struct to the buffer.
2. Moves your process back to the **Runnable Queue**.
3. The `read()` call returns the number of bytes written.

[Back to main README](../../README.md)
