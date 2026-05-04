package main

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

func TestEpollHandshake(t *testing.T) {
	stop := make(chan struct{})

	// Start server on a dynamic port
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	go func() {
		if err := runServer(port, stop); err != nil {
			fmt.Printf("Server exited with error: %v\n", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)

	// 1. Expect Banner
	banner, _ := reader.ReadString('\n')
	if strings.TrimSpace(banner) != "SERVER_READY" {
		t.Fatalf("expected SERVER_READY, got %q", banner)
	}

	// 2. Try invalid handshake
	fmt.Fprint(conn, "WRONG_CMD\n")
	errMsg, _ := reader.ReadString('\n')
	if !strings.HasPrefix(errMsg, "ERROR") {
		t.Fatalf("expected ERROR for invalid handshake, got %q", errMsg)
	}

	// 3. Send correct handshake
	fmt.Fprint(conn, "CLIENT_HELLO\n")
	authMsg, _ := reader.ReadString('\n')
	if !strings.HasPrefix(authMsg, "AUTH_SUCCESS") {
		t.Fatalf("expected AUTH_SUCCESS, got %q", authMsg)
	}

	// 4. Test Echo
	message := "Test Message\n"
	fmt.Fprint(conn, message)
	echoMsg, _ := reader.ReadString('\n')
	if echoMsg != "ECHO: Test Message\n" {
		t.Errorf("expected ECHO message, got %q", echoMsg)
	}

	close(stop)
	time.Sleep(100 * time.Millisecond)
}
