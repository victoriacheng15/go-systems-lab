package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
)

type eventInfo struct {
	mask uint32
	name string
}

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("Usage: %s <path>\n", os.Args[0])
		os.Exit(1)
	}
	targetPath, err := filepath.Abs(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving path: %v\n", err)
		os.Exit(1)
	}

	fd, err := syscall.InotifyInit()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing inotify: %v\n", err)
		os.Exit(1)
	}
	defer syscall.Close(fd)

	mask := syscall.IN_CREATE | syscall.IN_DELETE | syscall.IN_MODIFY |
		syscall.IN_MOVED_FROM | syscall.IN_MOVED_TO | syscall.IN_ATTRIB |
		syscall.IN_ACCESS | syscall.IN_OPEN | syscall.IN_CLOSE

	wd, err := syscall.InotifyAddWatch(fd, targetPath, uint32(mask))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error adding watch on %s: %v\n", targetPath, err)
		os.Exit(1)
	}

	fmt.Printf("--- Monitoring: %s (WD: %d) ---\n", targetPath, wd)

	var buf [syscall.SizeofInotifyEvent * 4096]byte

	for {
		n, err := syscall.Read(fd, buf[:])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading inotify events: %v\n", err)
			break
		}

		if n <= 0 {
			continue
		}

		events := parseEvents(buf[:n])
		for _, e := range events {
			var fullPath string
			if e.name == "" {
				fullPath = targetPath
			} else {
				fullPath = filepath.Join(targetPath, e.name)
			}
			printEvent(e.mask, fullPath)
		}
	}
}

func parseEvents(data []byte) []eventInfo {
	var events []eventInfo
	var offset uint32
	for offset < uint32(len(data)) {
		if uint32(len(data))-offset < syscall.SizeofInotifyEvent {
			break
		}
		event := (*syscall.InotifyEvent)(unsafe.Pointer(&data[offset]))

		var name string
		if event.Len > 0 {
			nameBytes := data[offset+syscall.SizeofInotifyEvent : offset+syscall.SizeofInotifyEvent+event.Len]
			for i, b := range nameBytes {
				if b == 0 {
					name = string(nameBytes[:i])
					break
				}
			}
		}

		events = append(events, eventInfo{mask: event.Mask, name: name})
		offset += syscall.SizeofInotifyEvent + event.Len
	}
	return events
}

func formatMask(mask uint32) string {
	switch {
	case mask&syscall.IN_ACCESS != 0:
		return "ACCESS/READ"
	case mask&syscall.IN_OPEN != 0:
		return "OPEN"
	case mask&syscall.IN_CLOSE_WRITE != 0:
		return "CLOSE_WRITE"
	case mask&syscall.IN_CLOSE_NOWRITE != 0:
		return "CLOSE_READ"
	case mask&syscall.IN_MODIFY != 0:
		return "MODIFY"
	case mask&syscall.IN_CREATE != 0:
		return "CREATE"
	case mask&syscall.IN_DELETE != 0:
		return "DELETE"
	case mask&syscall.IN_MOVED_FROM != 0:
		return "MOVE_OUT"
	case mask&syscall.IN_MOVED_TO != 0:
		return "MOVE_IN"
	case mask&syscall.IN_ATTRIB != 0:
		return "ATTRIB"
	default:
		return "EVENT"
	}
}

func printEvent(mask uint32, fullPath string) {
	typeStr := formatMask(mask)
	isDir := mask&syscall.IN_ISDIR != 0
	dirIndicator := ""
	if isDir {
		dirIndicator = "[DIR] "
	}

	fmt.Printf("[%s] %-12s | %s%s\n",
		time.Now().Format("15:04:05"),
		typeStr,
		dirIndicator,
		fullPath)
}
