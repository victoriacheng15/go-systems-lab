package main

import (
	"strings"
	"testing"
)

func TestParseJobStats(t *testing.T) {
	// A mock /proc/[pid]/stat line. RSS is the 24th field (index 23).
	// We want to verify that index 23 (1000 pages) results in ~3.9 MB (1000 * 4096 / 1024 / 1024)
	input := "1 (sh) S 0 1 1 0 -1 4194304 80 0 0 0 0 0 0 0 20 0 1 0 12345 4096000 1000"

	got, err := parseJobStats(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parseJobStats failed: %v", err)
	}

	expected := "RSS: 3 MB"
	if got != expected {
		t.Errorf("got %q, expected %q", got, expected)
	}
}

func TestParseBytes(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"50M", "52428800"},
		{"100K", "102400"},
		{"", "max"},
	}

	for _, tt := range tests {
		got := parseBytes(tt.input)
		if got != tt.expected {
			t.Errorf("parseBytes(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
