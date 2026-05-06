package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

const (
	prSetNoNewPrivs = 38
	prSetSeccomp    = 22

	seccompModeFilter = 2

	seccompRetKillProcess = 0x80000000
	seccompRetAllow       = 0x7fff0000

	auditArchX8664 = 0xc000003e

	bpfLD  = 0x00
	bpfW   = 0x00
	bpfABS = 0x20
	bpfJMP = 0x05
	bpfJEQ = 0x10
	bpfK   = 0x00
	bpfRET = 0x06
)

const (
	seccompDataNROffset   = 0
	seccompDataArchOffset = 4
)

var allowedSyscalls = []uintptr{
	syscall.SYS_READ,
	syscall.SYS_WRITE,
	syscall.SYS_CLOSE,
	syscall.SYS_EXIT,
	syscall.SYS_EXIT_GROUP,
	syscall.SYS_RT_SIGRETURN,
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "caps":
		caps, err := readCapabilities("/proc/self/status")
		if err != nil {
			fmt.Fprintf(os.Stderr, "read capabilities: %v\n", err)
			os.Exit(1)
		}
		printCapabilities(caps)
	case "sandbox":
		if len(os.Args) < 3 {
			usage()
			os.Exit(1)
		}
		runSandbox(os.Args[2])
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Println("Usage:")
	fmt.Println("  go run labs/07-seccomp-capabilities/main.go caps")
	fmt.Println("  go run labs/07-seccomp-capabilities/main.go sandbox allowed")
	fmt.Println("  go run labs/07-seccomp-capabilities/main.go sandbox breach")
}

func runSandbox(mode string) {
	runtime.LockOSThread()

	if err := installStrictSeccomp(); err != nil {
		fmt.Fprintf(os.Stderr, "install seccomp: %v\n", err)
		os.Exit(1)
	}

	switch mode {
	case "allowed":
		writeStdout("sandbox active: write(2) is allowed\n")
	case "breach":
		writeStdout("sandbox active: calling forbidden getpid(2)\n")
		syscall.RawSyscall(syscall.SYS_GETPID, 0, 0, 0)
		writeStdout("unreachable: forbidden syscall returned\n")
	default:
		writeStdout("unknown sandbox mode\n")
		os.Exit(1)
	}
}

func installStrictSeccomp() error {
	if _, _, errno := syscall.RawSyscall6(syscall.SYS_PRCTL, prSetNoNewPrivs, 1, 0, 0, 0, 0); errno != 0 {
		return errno
	}

	prog := buildSeccompProgram(allowedSyscalls)
	if _, _, errno := syscall.RawSyscall6(
		syscall.SYS_PRCTL,
		prSetSeccomp,
		seccompModeFilter,
		uintptr(unsafe.Pointer(&prog)),
		0,
		0,
		0,
	); errno != 0 {
		return errno
	}

	return nil
}

func buildSeccompProgram(allowed []uintptr) syscall.SockFprog {
	filters := make([]syscall.SockFilter, 0, 4+len(allowed)*2)

	filters = append(filters,
		stmt(bpfLD|bpfW|bpfABS, seccompDataArchOffset),
		jump(bpfJMP|bpfJEQ|bpfK, auditArchX8664, 1, 0),
		stmt(bpfRET|bpfK, seccompRetKillProcess),
		stmt(bpfLD|bpfW|bpfABS, seccompDataNROffset),
	)

	for _, nr := range allowed {
		filters = append(filters,
			jump(bpfJMP|bpfJEQ|bpfK, uint32(nr), 0, 1),
			stmt(bpfRET|bpfK, seccompRetAllow),
		)
	}

	filters = append(filters, stmt(bpfRET|bpfK, seccompRetKillProcess))

	return syscall.SockFprog{
		Len:    uint16(len(filters)),
		Filter: &filters[0],
	}
}

func stmt(code uint16, k uint32) syscall.SockFilter {
	return syscall.SockFilter{Code: code, K: k}
}

func jump(code uint16, k uint32, jt, jf uint8) syscall.SockFilter {
	return syscall.SockFilter{Code: code, Jt: jt, Jf: jf, K: k}
}

func writeStdout(msg string) {
	_, _, _ = syscall.RawSyscall(syscall.SYS_WRITE, uintptr(syscall.Stdout), uintptr(unsafe.Pointer(unsafe.StringData(msg))), uintptr(len(msg)))
}

type capabilitySet map[string]uint64

func readCapabilities(path string) (capabilitySet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseCapabilities(data)
}

func parseCapabilities(data []byte) (capabilitySet, error) {
	caps := capabilitySet{}
	for _, line := range bytes.Split(data, []byte{'\n'}) {
		key, value, ok := bytes.Cut(line, []byte{':'})
		if !ok || !bytes.HasPrefix(key, []byte("Cap")) {
			continue
		}

		parsed, err := strconv.ParseUint(strings.TrimSpace(string(value)), 16, 64)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", key, err)
		}
		caps[string(key)] = parsed
	}

	if len(caps) == 0 {
		return nil, errors.New("no capability fields found")
	}

	return caps, nil
}

func printCapabilities(caps capabilitySet) {
	for _, key := range []string{"CapInh", "CapPrm", "CapEff", "CapBnd", "CapAmb"} {
		value, ok := caps[key]
		if !ok {
			continue
		}
		fmt.Printf("%s: 0x%016x (%064b)\n", key, value, value)
	}
}

func hostEndian() binary.ByteOrder {
	var x uint16 = 0x1
	if *(*byte)(unsafe.Pointer(&x)) == 0x1 {
		return binary.LittleEndian
	}
	return binary.BigEndian
}
