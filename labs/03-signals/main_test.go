package main

import (
	"context"
	"os"
	"syscall"
	"testing"
)

func TestHandleSignal(t *testing.T) {
	tests := []struct {
		name         string
		sig          os.Signal
		initialDebug bool
		wantExit     bool
		wantCancel   bool
		wantDebug    bool
	}{
		{
			name:         "SIGHUP Toggles Debug On",
			sig:          syscall.SIGHUP,
			initialDebug: false,
			wantExit:     false,
			wantCancel:   false,
			wantDebug:    true,
		},
		{
			name:         "Shutdown on SIGTERM",
			sig:          syscall.SIGTERM,
			initialDebug: false,
			wantExit:     true,
			wantCancel:   true,
			wantDebug:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary file for the test to satisfy the signature
			tmpFile, err := os.CreateTemp("", "signal_test_*.log")
			if err != nil {
				t.Fatalf("failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())
			defer tmpFile.Close()

			cfg := &config{debug: tt.initialDebug}
			ctx, cancel := context.WithCancel(context.Background())

			// Pass the tmpFile to the updated handleSignal signature
			gotExit := handleSignal(tt.sig, cfg, cancel, tmpFile)

			if gotExit != tt.wantExit {
				t.Errorf("handleSignal() gotExit = %v, want %v", gotExit, tt.wantExit)
			}

			if cfg.debug != tt.wantDebug {
				t.Errorf("handleSignal() debug = %v, want %v", cfg.debug, tt.wantDebug)
			}

			cancelled := false
			select {
			case <-ctx.Done():
				cancelled = true
			default:
			}

			if cancelled != tt.wantCancel {
				t.Errorf("handleSignal() cancelled = %v, want %v", cancelled, tt.wantCancel)
			}
		})
	}
}
