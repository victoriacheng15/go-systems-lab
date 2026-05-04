package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

const (
	MaxEvents = 64
	Port      = 8080
)

// Connection State
const (
	StateHandshake = iota
	StateAuthenticated
)

type connContext struct {
	state int
}

func main() {
	stop := make(chan struct{})
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs
		fmt.Println("\n>> Received shutdown signal...")
		close(stop)
	}()

	if err := runServer(Port, stop); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}

func runServer(port int, stop <-chan struct{}) error {
	fmt.Printf("--- epoll Systems Lab: Stateful Protocol Server ---\n")
	fmt.Printf("Listening on: localhost:%d\n", port)
	fmt.Println("Protocol: Connect -> Wait for SERVER_READY -> Send CLIENT_HELLO -> Start Echoing")

	// 1. Create a Listening TCP Socket
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("error creating listener: %w", err)
	}
	defer listener.Close()

	file, err := listener.(*net.TCPListener).File()
	if err != nil {
		return fmt.Errorf("error getting listener file: %w", err)
	}
	listenFd := int(file.Fd())

	if err := syscall.SetNonblock(listenFd, true); err != nil {
		return fmt.Errorf("error setting non-blocking: %w", err)
	}

	// 2. Initialize epoll
	epfd, err := syscall.EpollCreate1(0)
	if err != nil {
		return fmt.Errorf("error creating epoll: %w", err)
	}
	defer syscall.Close(epfd)

	event := &syscall.EpollEvent{
		Events: syscall.EPOLLIN,
		Fd:     int32(listenFd),
	}
	if err := syscall.EpollCtl(epfd, syscall.EPOLL_CTL_ADD, listenFd, event); err != nil {
		return fmt.Errorf("error adding listener to epoll: %w", err)
	}

	// 4. Main Event Loop
	events := make([]syscall.EpollEvent, MaxEvents)
	connections := make(map[int]*connContext)

	for {
		n, err := syscall.EpollWait(epfd, events, 100)
		if err != nil && err != syscall.EINTR {
			return fmt.Errorf("error in epoll_wait: %w", err)
		}

		select {
		case <-stop:
			fmt.Println(">> INITIATING GRACEFUL SHUTDOWN (epoll loop)")
			for fd := range connections {
				syscall.Close(fd)
			}
			return nil
		default:
		}

		for i := 0; i < n; i++ {
			fd := int(events[i].Fd)

			if fd == listenFd {
				// Handle New Connection
				conn, err := listener.Accept()
				if err != nil {
					continue
				}

				tcpConn := conn.(*net.TCPConn)
				f, _ := tcpConn.File()
				cfd := int(f.Fd())

				syscall.SetNonblock(cfd, true)

				clientEvent := &syscall.EpollEvent{
					Events: syscall.EPOLLIN,
					Fd:     int32(cfd),
				}
				syscall.EpollCtl(epfd, syscall.EPOLL_CTL_ADD, cfd, clientEvent)
				
				// Initialize connection context
				connections[cfd] = &connContext{state: StateHandshake}
				
				// Send Banner (the start of the handshake)
				syscall.Write(cfd, []byte("SERVER_READY\n"))
				fmt.Printf("[%d] NEW CONNECTION -> State: Handshake\n", cfd)

			} else {
				// Handle Data
				ctx, ok := connections[fd]
				if !ok {
					continue
				}

				buf := make([]byte, 1024)
				rn, err := syscall.Read(fd, buf)

				if rn <= 0 || err != nil {
					fmt.Printf("[%d] CONNECTION CLOSED\n", fd)
					syscall.EpollCtl(epfd, syscall.EPOLL_CTL_DEL, fd, nil)
					syscall.Close(fd)
					delete(connections, fd)
					continue
				}

				input := strings.TrimSpace(string(buf[:rn]))

				if ctx.state == StateHandshake {
					if input == "CLIENT_HELLO" {
						ctx.state = StateAuthenticated
						syscall.Write(fd, []byte("AUTH_SUCCESS: Welcome to Echo Room\n"))
						fmt.Printf("[%d] HANDSHAKE COMPLETE -> State: Authenticated\n", fd)
					} else {
						syscall.Write(fd, []byte("ERROR: Send CLIENT_HELLO to begin\n"))
						fmt.Printf("[%d] INVALID HANDSHAKE: %q\n", fd, input)
					}
				} else {
					// Authenticated: Standard Echo
					fmt.Printf("[%d] ECHO: %s\n", fd, input)
					syscall.Write(fd, []byte("ECHO: "))
					syscall.Write(fd, buf[:rn])
				}
			}
		}
	}
}
