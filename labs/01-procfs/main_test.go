package main

import (
	"strings"
	"testing"
)

func TestParseCPULoad(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "valid loadavg",
			input:    "0.05 0.12 0.08 1/120 12345",
			expected: "1 min:  0.05 procs\n5 min:  0.12 procs\n15 min: 0.08 procs",
			wantErr:  false,
		},
		{
			name:     "invalid format",
			input:    "too_few_fields",
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.input)
			got, err := parseCPULoad(r)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseCPULoad() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("got:\n%s\nexpected:\n%s", got, tt.expected)
			}
		})
	}
}

func TestParseCPUSamples(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantCores []string
		checkVal  func(*testing.T, map[string]coreStats)
	}{
		{
			name:      "valid stat",
			input:     "cpu  2255 34 2290 22625 6290 127 456\ncpu0 1132 17 1150 11312 3145 63 228",
			wantCores: []string{"cpu0"},
			checkVal: func(t *testing.T, samples map[string]coreStats) {
				if _, ok := samples["cpu"]; ok {
					t.Errorf("samples should not contain aggregate 'cpu'")
				}
				if c0, ok := samples["cpu0"]; !ok || c0.user != 1132 {
					t.Errorf("cpu0.user got %d, expected 1132", c0.user)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.input)
			samples, err := parseCPUSamples(r)
			if err != nil {
				t.Fatalf("parseCPUSamples failed: %v", err)
			}
			tt.checkVal(t, samples)
		})
	}
}

func TestParseNetSamples(t *testing.T) {
	tests := []struct {
		name  string
		input string
		iface string
		wantRX uint64
		wantTX uint64
	}{
		{
			name:  "valid net dev",
			input: "Inter-|   Receive                                                |  Transmit\n face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed\n  eth0: 1000 10 0 0 0 0 0 0 2000 20 0 0 0 0 0 0",
			iface: "eth0",
			wantRX: 1000,
			wantTX: 2000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.input)
			samples, err := parseNetSamples(r)
			if err != nil {
				t.Fatalf("parseNetSamples failed: %v", err)
			}

			s, ok := samples[tt.iface]
			if !ok {
				t.Fatalf("%s not found in samples", tt.iface)
			}

			if s.rxBytes != tt.wantRX || s.txBytes != tt.wantTX {
				t.Errorf("%s stats mismatch: rx=%d (want %d), tx=%d (want %d)", tt.iface, s.rxBytes, tt.wantRX, s.txBytes, tt.wantTX)
			}
		})
	}
}

func TestParseMemInfo(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		limit          int
		wantInOutput   string
		wantLineCount  int
	}{
		{
			name:          "valid meminfo",
			input:         "MemTotal:       8010836 kB\nMemFree:         123456 kB",
			limit:         2,
			wantInOutput:  "7823.1 MB",
			wantLineCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.input)
			out, err := parseMemInfo(r, tt.limit)
			if err != nil {
				t.Fatalf("parseMemInfo failed: %v", err)
			}

			if len(out) != tt.wantLineCount {
				t.Fatalf("expected %d lines, got %d", tt.wantLineCount, len(out))
			}

			found := false
			for _, line := range out {
				if strings.Contains(line, tt.wantInOutput) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected string %q not found in output", tt.wantInOutput)
			}
		})
	}
}
