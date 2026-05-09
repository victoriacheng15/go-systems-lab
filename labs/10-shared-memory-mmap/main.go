package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	scratchDir  = "labs/10-shared-memory-mmap/scratch"
	scratchFile = scratchDir + "/shared-memory.bin"

	mappingSize   = 4096
	magicValue    = "MMAPIPC1"
	layoutVersion = uint32(1)

	magicOffset   = 0
	versionOffset = 8
	seqOffset     = 16
	timeOffset    = 24
	lenOffset     = 32
	payloadOffset = 64
)

var errInvalidMapping = errors.New("invalid shared memory mapping")

type sharedMessage struct {
	Sequence  uint64
	Timestamp time.Time
	Payload   string
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "init":
		if err := runInit(); err != nil {
			fmt.Fprintf(os.Stderr, "init: %v\n", err)
			os.Exit(1)
		}
	case "inspect":
		if err := runInspect(); err != nil {
			fmt.Fprintf(os.Stderr, "inspect: %v\n", err)
			os.Exit(1)
		}
	case "write":
		if err := runWrite(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "write: %v\n", err)
			os.Exit(1)
		}
	case "read":
		if err := runRead(); err != nil {
			fmt.Fprintf(os.Stderr, "read: %v\n", err)
			os.Exit(1)
		}
	case "watch":
		if err := runWatch(); err != nil {
			fmt.Fprintf(os.Stderr, "watch: %v\n", err)
			os.Exit(1)
		}
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Println("Usage:")
	fmt.Println("  go run labs/10-shared-memory-mmap/main.go init")
	fmt.Println("  go run labs/10-shared-memory-mmap/main.go inspect")
	fmt.Println("  go run labs/10-shared-memory-mmap/main.go write [message]")
	fmt.Println("  go run labs/10-shared-memory-mmap/main.go read")
	fmt.Println("  go run labs/10-shared-memory-mmap/main.go watch")
}

func runInit() error {
	mapping, closeMapping, err := openSharedMapping(true)
	if err != nil {
		return err
	}
	defer closeMapping()

	initMapping(mapping)
	if err := syncMapping(mapping); err != nil {
		return err
	}

	fmt.Printf("initialized shared mapping at %s (%d bytes)\n", scratchFile, mappingSize)
	return nil
}

func runInspect() error {
	mapping, closeMapping, err := openSharedMapping(true)
	if err != nil {
		return err
	}
	defer closeMapping()

	if !hasValidHeader(mapping) {
		initMapping(mapping)
	}

	start, end := mappingAddressRange(mapping)
	fmt.Printf("mapped file: %s\n", scratchFile)
	fmt.Printf("virtual address range: %#x-%#x (%d bytes)\n", start, end, len(mapping))

	entries, err := findCurrentProcessMappings(scratchFile)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Println("/proc/self/maps entry: not found")
		return nil
	}

	fmt.Println("/proc/self/maps entry:")
	for _, entry := range entries {
		fmt.Println(entry)
	}

	return nil
}

func runWrite(args []string) error {
	message := "hello through mmap"
	if len(args) > 0 {
		message = strings.Join(args, " ")
	}

	mapping, closeMapping, err := openSharedMapping(true)
	if err != nil {
		return err
	}
	defer closeMapping()

	if !hasValidHeader(mapping) {
		initMapping(mapping)
	}

	written, err := writeMessage(mapping, message, time.Now())
	if err != nil {
		return err
	}
	if err := syncMapping(mapping); err != nil {
		return err
	}

	fmt.Printf("wrote seq=%d len=%d to %s\n", written.Sequence, len(written.Payload), scratchFile)
	return nil
}

func runRead() error {
	mapping, closeMapping, err := openSharedMapping(false)
	if err != nil {
		return err
	}
	defer closeMapping()

	message, err := readMessage(mapping)
	if err != nil {
		return err
	}

	printMessage(message)
	return nil
}

func runWatch() error {
	mapping, closeMapping, err := openSharedMapping(false)
	if err != nil {
		return err
	}
	defer closeMapping()

	fmt.Printf("watching %s\n", scratchFile)
	var lastSeq uint64
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		message, err := readMessage(mapping)
		if err != nil {
			return err
		}
		if message.Sequence == 0 || message.Sequence == lastSeq {
			continue
		}
		lastSeq = message.Sequence
		printMessage(message)
	}

	return nil
}

func openSharedMapping(create bool) ([]byte, func() error, error) {
	flag := os.O_RDWR
	if create {
		flag |= os.O_CREATE
	}

	if err := os.MkdirAll(scratchDir, 0755); err != nil {
		return nil, nil, err
	}

	file, err := os.OpenFile(filepath.Clean(scratchFile), flag, 0644)
	if err != nil {
		return nil, nil, err
	}

	if create {
		if err := file.Truncate(mappingSize); err != nil {
			file.Close()
			return nil, nil, err
		}
	}

	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, nil, err
	}
	if info.Size() < mappingSize {
		file.Close()
		return nil, nil, fmt.Errorf("%s is %d bytes; run init first", scratchFile, info.Size())
	}

	mapping, err := syscall.Mmap(int(file.Fd()), 0, mappingSize, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		file.Close()
		return nil, nil, err
	}

	closeMapping := func() error {
		munmapErr := syscall.Munmap(mapping)
		closeErr := file.Close()
		if munmapErr != nil {
			return munmapErr
		}
		return closeErr
	}

	return mapping, closeMapping, nil
}

func initMapping(mapping []byte) {
	clear(mapping)
	copy(mapping[magicOffset:magicOffset+len(magicValue)], magicValue)
	binary.LittleEndian.PutUint32(mapping[versionOffset:], layoutVersion)
}

func writeMessage(mapping []byte, payload string, now time.Time) (sharedMessage, error) {
	if !hasValidHeader(mapping) {
		return sharedMessage{}, errInvalidMapping
	}
	if len(payload) > maxPayloadSize() {
		return sharedMessage{}, fmt.Errorf("payload is %d bytes; max is %d", len(payload), maxPayloadSize())
	}

	seq := binary.LittleEndian.Uint64(mapping[seqOffset:]) + 1
	binary.LittleEndian.PutUint64(mapping[seqOffset:], seq)
	binary.LittleEndian.PutUint64(mapping[timeOffset:], uint64(now.UnixNano()))
	binary.LittleEndian.PutUint32(mapping[lenOffset:], uint32(len(payload)))

	payloadArea := mapping[payloadOffset:]
	clear(payloadArea)
	copy(payloadArea, payload)

	return sharedMessage{Sequence: seq, Timestamp: now, Payload: payload}, nil
}

func readMessage(mapping []byte) (sharedMessage, error) {
	if !hasValidHeader(mapping) {
		return sharedMessage{}, errInvalidMapping
	}

	length := binary.LittleEndian.Uint32(mapping[lenOffset:])
	if int(length) > maxPayloadSize() {
		return sharedMessage{}, fmt.Errorf("payload length %d exceeds mapping payload area", length)
	}

	nanos := int64(binary.LittleEndian.Uint64(mapping[timeOffset:]))
	return sharedMessage{
		Sequence:  binary.LittleEndian.Uint64(mapping[seqOffset:]),
		Timestamp: time.Unix(0, nanos),
		Payload:   string(mapping[payloadOffset : payloadOffset+int(length)]),
	}, nil
}

func hasValidHeader(mapping []byte) bool {
	if len(mapping) < mappingSize {
		return false
	}
	if string(mapping[magicOffset:magicOffset+len(magicValue)]) != magicValue {
		return false
	}
	return binary.LittleEndian.Uint32(mapping[versionOffset:]) == layoutVersion
}

func syncMapping(mapping []byte) error {
	if len(mapping) == 0 {
		return nil
	}

	_, _, errno := syscall.Syscall(syscall.SYS_MSYNC, uintptr(unsafe.Pointer(&mapping[0])), uintptr(len(mapping)), uintptr(syscall.MS_SYNC))
	if errno != 0 {
		return errno
	}
	return nil
}

func maxPayloadSize() int {
	return mappingSize - payloadOffset
}

func mappingAddressRange(mapping []byte) (uintptr, uintptr) {
	if len(mapping) == 0 {
		return 0, 0
	}
	start := uintptr(unsafe.Pointer(&mapping[0]))
	return start, start + uintptr(len(mapping))
}

func findCurrentProcessMappings(path string) ([]string, error) {
	absolutePath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return nil, err
	}

	content, err := os.ReadFile("/proc/self/maps")
	if err != nil {
		return nil, err
	}

	var entries []string
	for _, line := range strings.Split(string(content), "\n") {
		if strings.Contains(line, absolutePath) {
			entries = append(entries, line)
		}
	}
	return entries, nil
}

func printMessage(message sharedMessage) {
	fmt.Printf("seq=%d time=%s payload=%q\n", message.Sequence, message.Timestamp.Format(time.RFC3339Nano), message.Payload)
}
