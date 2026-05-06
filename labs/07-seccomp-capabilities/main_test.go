package main

import (
	"strings"
	"syscall"
	"testing"
	"unsafe"
)

func TestParseCapabilities(t *testing.T) {
	input := []byte(strings.Join([]string{
		"Name:\tseccomp-lab",
		"CapInh:\t0000000000000000",
		"CapPrm:\t00000000a80425fb",
		"CapEff:\t00000000a80425fb",
		"CapBnd:\t00000000a80425fb",
		"CapAmb:\t0000000000000000",
	}, "\n"))

	caps, err := parseCapabilities(input)
	if err != nil {
		t.Fatalf("parseCapabilities returned error: %v", err)
	}

	if caps["CapEff"] != 0xa80425fb {
		t.Fatalf("CapEff = %#x, want %#x", caps["CapEff"], uint64(0xa80425fb))
	}
	if caps["CapAmb"] != 0 {
		t.Fatalf("CapAmb = %#x, want 0", caps["CapAmb"])
	}
}

func TestParseCapabilitiesRejectsBadHex(t *testing.T) {
	_, err := parseCapabilities([]byte("CapEff:\tnot-hex\n"))
	if err == nil {
		t.Fatal("parseCapabilities succeeded for invalid hex")
	}
}

func TestBuildSeccompProgram(t *testing.T) {
	prog := buildSeccompProgram([]uintptr{syscall.SYS_WRITE, syscall.SYS_EXIT_GROUP})

	if prog.Len != 9 {
		t.Fatalf("program length = %d, want 9", prog.Len)
	}
	if prog.Filter == nil {
		t.Fatal("program filter pointer is nil")
	}

	filters := unsafe.Slice(prog.Filter, prog.Len)
	last := filters[len(filters)-1]
	if last.Code != bpfRET|bpfK {
		t.Fatalf("last instruction code = %#x, want RET", last.Code)
	}
	if last.K != seccompRetKillProcess {
		t.Fatalf("last instruction action = %#x, want kill process", last.K)
	}
}

func TestHostEndian(t *testing.T) {
	if hostEndian() == nil {
		t.Fatal("hostEndian returned nil")
	}
}
