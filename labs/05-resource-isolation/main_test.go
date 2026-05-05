package main

import (
	"testing"
)

func TestParseBytes(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"50M", "52428800"},
		{"1G", "1073741824"},
		{"100K", "102400"},
		{"max", "max"},
		{"", "max"},
		{"100", "100"},
	}

	for _, tt := range tests {
		got := parseBytes(tt.input)
		if got != tt.expected {
			t.Errorf("parseBytes(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
