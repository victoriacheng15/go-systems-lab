# 04 epoll

`epoll` is a scalable I/O event notification facility in the Linux kernel. This lab demonstrates a **Stateful Network Orchestrator** using a single-threaded event loop.

## What It Demonstrates

- **The Event Loop**: Moving from "One Thread per Connection" to "One Thread for Thousands of Connections."
- **Stateful Protocols**: Implementing a **Handshake** mechanism where the server tracks the "Authentication State" of each individual file descriptor.
- **Non-blocking I/O**: Understanding how to tell the kernel to "Wait and Notify" instead of "Block and Sleep."
- **Resource Management**: Efficiently handling new connections and data events without spawning thousands of goroutines.

## Manual Usage

Run from the repository root:

1. **Start the epoll server:**
   ```bash
   go run labs/04-epoll/main.go
   ```

2. **Connect from another terminal:**
   ```bash
   nc localhost 8080
   ```

3. **Complete the Handshake:**
   - Server will send: `SERVER_READY`
   - You must type: `CLIENT_HELLO`
   - Server will respond: `AUTH_SUCCESS`

4. **Start Echoing:** After the handshake, any message you type will be echoed back.

## 📖 Reference: How epoll Works

In the old days, servers used `select()` or `poll()`, which forced the kernel to scan every single connection every time a single packet arrived. `epoll` is $O(1)$ because the kernel maintains a list of *only* the active events.

### The Stateful Loop
In this lab, the server doesn't just "Echo." It maintains a `connContext` for every FD. 
1. **`epoll_wait()`** notifies us an FD has data.
2. We look up the FD in our **Connection Map**.
3. We check the **State** (Handshake vs. Authenticated).
4. We apply different logic based on that state.

[Back to main README](../../README.md)
