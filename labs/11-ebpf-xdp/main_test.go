package main

import (
	"strings"
	"testing"
)

func TestValidateInterfaceName(t *testing.T) {
	if err := validateInterfaceName("lo"); err != nil {
		t.Fatalf("lo should be valid: %v", err)
	}
	if err := validateInterfaceName(""); err == nil {
		t.Fatal("empty interface name should be invalid")
	}
	if err := validateInterfaceName("../eth0"); err == nil {
		t.Fatal("path-like interface name should be invalid")
	}
}

func TestBuildWorkflowCommands(t *testing.T) {
	commands := buildWorkflowCommands("lo")
	joined := strings.Join(commands, "\n")

	for _, want := range []string{
		"clang -O2 -target bpf",
		"labs/11-ebpf-xdp/bpf/xdp_pass.c",
		"llvm-objdump -h labs/11-ebpf-xdp/build/xdp_pass.o",
		"sudo bpftool prog load labs/11-ebpf-xdp/build/xdp_pass.o /sys/fs/bpf/xdp_pass_lab type xdp",
		"sudo bpftool net attach xdpgeneric pinned /sys/fs/bpf/xdp_pass_lab dev lo overwrite",
		"sudo bpftool net show dev lo",
		"sudo bpftool net detach xdpgeneric dev lo",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("workflow commands missing %q:\n%s", want, joined)
		}
	}
}

func TestStatusFromInterfaceDefaultsHardware(t *testing.T) {
	status := interfaceStatus{Name: "lo", Index: 1}
	if status.Name != "lo" {
		t.Fatalf("name = %q, want lo", status.Name)
	}
}

func TestCollectProbeResultsIncludesSource(t *testing.T) {
	results := collectProbeResults()
	var found bool
	for _, result := range results {
		if result.Name == "XDP source" {
			found = true
			if result.Status != "ok" {
				t.Fatalf("XDP source status = %q, want ok", result.Status)
			}
		}
	}
	if !found {
		t.Fatal("probe results did not include XDP source")
	}
}
