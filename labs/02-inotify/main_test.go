package main

import (
	"encoding/binary"
	"syscall"
	"testing"
)

func TestParseEvents(t *testing.T) {
	tests := []struct {
		name     string
		mask     uint32
		fileName string
	}{
		{
			name:     "Single File Modify",
			mask:     syscall.IN_MODIFY,
			fileName: "test.txt",
		},
		{
			name:     "Directory Create",
			mask:     syscall.IN_CREATE | syscall.IN_ISDIR,
			fileName: "new_dir",
		},
		{
			name:     "Self Access (no name)",
			mask:     syscall.IN_ACCESS,
			fileName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Construct raw inotify_event bytes
			// struct inotify_event {
			//     int      wd;
			//     uint32_t mask;
			//     uint32_t cookie;
			//     uint32_t len;
			//     char     name[];
			// };

			nameLen := uint32(0)
			if tt.fileName != "" {
				// Name must be null-terminated and padded to align with the struct
				nameLen = uint32(len(tt.fileName) + 1)
				// Pad to 16 bytes alignment (common for inotify)
				if nameLen%16 != 0 {
					nameLen += 16 - (nameLen % 16)
				}
			}

			buf := make([]byte, syscall.SizeofInotifyEvent+int(nameLen))
			binary.LittleEndian.PutUint32(buf[0:4], 1)         // wd
			binary.LittleEndian.PutUint32(buf[4:8], tt.mask)   // mask
			binary.LittleEndian.PutUint32(buf[8:12], 0)        // cookie
			binary.LittleEndian.PutUint32(buf[12:16], nameLen) // len

			if tt.fileName != "" {
				copy(buf[16:], tt.fileName)
			}

			events := parseEvents(buf)
			if len(events) != 1 {
				t.Fatalf("expected 1 event, got %d", len(events))
			}

			if events[0].mask != tt.mask {
				t.Errorf("expected mask %x, got %x", tt.mask, events[0].mask)
			}

			if events[0].name != tt.fileName {
				t.Errorf("expected name %q, got %q", tt.fileName, events[0].name)
			}
		})
	}
}

func TestFormatMask(t *testing.T) {
	tests := []struct {
		mask     uint32
		expected string
	}{
		{syscall.IN_ACCESS, "ACCESS/READ"},
		{syscall.IN_MODIFY, "MODIFY"},
		{syscall.IN_CREATE, "CREATE"},
		{syscall.IN_DELETE, "DELETE"},
		{0, "EVENT"},
	}

	for _, tt := range tests {
		got := formatMask(tt.mask)
		if got != tt.expected {
			t.Errorf("formatMask(%x) = %q; want %q", tt.mask, got, tt.expected)
		}
	}
}
