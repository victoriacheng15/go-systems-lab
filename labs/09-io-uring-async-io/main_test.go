package main

import (
	"errors"
	"strings"
	"syscall"
	"testing"
	"unsafe"
)

func TestStructSizesMatchKernelABI(t *testing.T) {
	if got := unsafe.Sizeof(ioUringSQE{}); got != sqeSize {
		t.Fatalf("SQE size = %d, want %d", got, sqeSize)
	}
	if got := unsafe.Sizeof(ioUringCQE{}); got != cqeSize {
		t.Fatalf("CQE size = %d, want %d", got, cqeSize)
	}
}

func TestRingOffsetHelpers(t *testing.T) {
	buf := make([]byte, 16)

	storeUint32(buf, 4, 42)
	if got := loadUint32(buf, 4); got != 42 {
		t.Fatalf("loadUint32 = %d, want 42", got)
	}
}

func TestSQEAndCQEIndexing(t *testing.T) {
	sqes := make([]byte, sqeSize*2)
	sqe := sqeAt(sqes, 1)
	sqe.Opcode = ioUringOpNOP
	sqe.UserData = 0x1234

	if got := sqeAt(sqes, 1).UserData; got != 0x1234 {
		t.Fatalf("SQE user data = %#x, want 0x1234", got)
	}

	cq := make([]byte, 64)
	base := uint32(16)
	cqe := cqeAt(cq, base, 1)
	cqe.UserData = 0xabcd
	cqe.Res = 0

	if got := cqeAt(cq, base, 1).UserData; got != 0xabcd {
		t.Fatalf("CQE user data = %#x, want 0xabcd", got)
	}
}

func TestDescribeSetupError(t *testing.T) {
	err := describeSetupError(syscall.EPERM)
	if !errors.Is(err, syscall.EPERM) {
		t.Fatalf("wrapped error does not preserve EPERM: %v", err)
	}
	if !strings.Contains(err.Error(), "blocked") {
		t.Fatalf("error %q does not explain blocked setup", err)
	}
}

func TestMaxInt(t *testing.T) {
	if got := maxInt(2, 5); got != 5 {
		t.Fatalf("maxInt(2, 5) = %d, want 5", got)
	}
	if got := maxInt(7, 3); got != 7 {
		t.Fatalf("maxInt(7, 3) = %d, want 7", got)
	}
}
