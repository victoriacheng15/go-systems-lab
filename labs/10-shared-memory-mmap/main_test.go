package main

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestInitMappingWritesHeader(t *testing.T) {
	mapping := make([]byte, mappingSize)
	initMapping(mapping)

	if !hasValidHeader(mapping) {
		t.Fatal("mapping header was not valid after init")
	}
}

func TestWriteAndReadMessage(t *testing.T) {
	mapping := make([]byte, mappingSize)
	initMapping(mapping)

	now := time.Unix(1710000000, 123)
	written, err := writeMessage(mapping, "hello mmap", now)
	if err != nil {
		t.Fatalf("writeMessage returned error: %v", err)
	}
	if written.Sequence != 1 {
		t.Fatalf("sequence = %d, want 1", written.Sequence)
	}

	read, err := readMessage(mapping)
	if err != nil {
		t.Fatalf("readMessage returned error: %v", err)
	}
	if read.Sequence != 1 {
		t.Fatalf("read sequence = %d, want 1", read.Sequence)
	}
	if read.Payload != "hello mmap" {
		t.Fatalf("payload = %q, want %q", read.Payload, "hello mmap")
	}
	if !read.Timestamp.Equal(now) {
		t.Fatalf("timestamp = %s, want %s", read.Timestamp, now)
	}
}

func TestWriteIncrementsSequence(t *testing.T) {
	mapping := make([]byte, mappingSize)
	initMapping(mapping)

	if _, err := writeMessage(mapping, "one", time.Now()); err != nil {
		t.Fatalf("first write failed: %v", err)
	}
	second, err := writeMessage(mapping, "two", time.Now())
	if err != nil {
		t.Fatalf("second write failed: %v", err)
	}

	if second.Sequence != 2 {
		t.Fatalf("sequence = %d, want 2", second.Sequence)
	}
}

func TestInvalidMapping(t *testing.T) {
	_, err := readMessage(make([]byte, mappingSize))
	if !errors.Is(err, errInvalidMapping) {
		t.Fatalf("readMessage error = %v, want errInvalidMapping", err)
	}
}

func TestPayloadLimit(t *testing.T) {
	mapping := make([]byte, mappingSize)
	initMapping(mapping)

	_, err := writeMessage(mapping, strings.Repeat("x", maxPayloadSize()+1), time.Now())
	if err == nil {
		t.Fatal("expected payload limit error")
	}
}

func TestMappingAddressRange(t *testing.T) {
	mapping := make([]byte, mappingSize)
	start, end := mappingAddressRange(mapping)

	if start == 0 {
		t.Fatal("start address should not be zero")
	}
	if got := end - start; got != mappingSize {
		t.Fatalf("address range size = %d, want %d", got, mappingSize)
	}
}
