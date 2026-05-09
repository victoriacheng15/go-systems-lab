package main

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

const (
	sysIOUringSetup = 425
	sysIOUringEnter = 426

	ioUringOpNOP    = 0
	ioUringOpReadV  = 1
	ioUringOpWriteV = 2

	ioUringEnterGetEvents = 1 << 0

	ioUringFeatSingleMmap = 1 << 0

	ioUringOffSQRing = 0
	ioUringOffCQRing = 0x8000000
	ioUringOffSQEs   = 0x10000000

	sqeSize = 64
	cqeSize = 16

	scratchDir  = "labs/09-io-uring-async-io/scratch"
	scratchFile = scratchDir + "/io-uring-lab.txt"
)

type ioSqringOffsets struct {
	Head        uint32
	Tail        uint32
	RingMask    uint32
	RingEntries uint32
	Flags       uint32
	Dropped     uint32
	Array       uint32
	Resv1       uint32
	UserAddr    uint64
}

type ioCqringOffsets struct {
	Head        uint32
	Tail        uint32
	RingMask    uint32
	RingEntries uint32
	Overflow    uint32
	CQEs        uint32
	Flags       uint32
	Resv1       uint32
	UserAddr    uint64
}

type ioUringParams struct {
	SQEntries    uint32
	CQEntries    uint32
	Flags        uint32
	SQThreadCPU  uint32
	SQThreadIdle uint32
	Features     uint32
	WQFD         uint32
	Resv         [3]uint32
	SQOff        ioSqringOffsets
	CQOff        ioCqringOffsets
}

type ioUringSQE struct {
	Opcode      uint8
	Flags       uint8
	IOPrio      uint16
	FD          int32
	Off         uint64
	Addr        uint64
	Len         uint32
	RWFlags     uint32
	UserData    uint64
	BufIndex    uint16
	Personality uint16
	SpliceFDIn  int32
	Addr3       uint64
	Pad2        uint64
}

type ioUringCQE struct {
	UserData uint64
	Res      int32
	Flags    uint32
}

type ringMapping struct {
	fd     int
	params ioUringParams
	sqRing []byte
	cqRing []byte
	sqes   []byte
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "probe":
		if err := probe(); err != nil {
			fmt.Fprintf(os.Stderr, "probe: %v\n", err)
			os.Exit(1)
		}
	case "nop":
		if err := runNOP(); err != nil {
			fmt.Fprintf(os.Stderr, "nop: %v\n", err)
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
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Println("Usage:")
	fmt.Println("  go run labs/09-io-uring-async-io/main.go probe")
	fmt.Println("  go run labs/09-io-uring-async-io/main.go nop")
	fmt.Println("  go run labs/09-io-uring-async-io/main.go write [message]")
	fmt.Println("  go run labs/09-io-uring-async-io/main.go read")
}

func probe() error {
	fd, params, err := setupRing(8)
	if err != nil {
		return describeSetupError(err)
	}
	defer syscall.Close(fd)

	fmt.Printf("io_uring available: sq_entries=%d cq_entries=%d features=0x%x\n", params.SQEntries, params.CQEntries, params.Features)
	if params.Features&ioUringFeatSingleMmap != 0 {
		fmt.Println("feature: IORING_FEAT_SINGLE_MMAP")
	}
	return nil
}

func runNOP() error {
	ring, err := mapRing(8)
	if err != nil {
		return describeSetupError(err)
	}
	defer ring.close()

	const userData = 0xabcddcba
	cqe, err := ring.submitNOP(userData)
	if err != nil {
		return err
	}
	if cqe.UserData != userData {
		return fmt.Errorf("completion user_data=%#x, want %#x", cqe.UserData, uint64(userData))
	}
	if cqe.Res < 0 {
		return fmt.Errorf("completion result=%d", cqe.Res)
	}

	fmt.Printf("completed IORING_OP_NOP: user_data=%#x res=%d flags=0x%x\n", cqe.UserData, cqe.Res, cqe.Flags)
	return nil
}

func runWrite(args []string) error {
	message := "hello from io_uring\n"
	if len(args) > 0 {
		message = args[0] + "\n"
	}

	if err := os.MkdirAll(scratchDir, 0755); err != nil {
		return err
	}

	file, err := os.OpenFile(scratchFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	ring, err := mapRing(8)
	if err != nil {
		return describeSetupError(err)
	}
	defer ring.close()

	written, err := ring.writeAt(file.Fd(), []byte(message), 0)
	if err != nil {
		return err
	}
	if written != len(message) {
		return fmt.Errorf("short write: wrote %d bytes, want %d", written, len(message))
	}

	fmt.Printf("wrote %d bytes through IORING_OP_WRITEV to %s\n", written, scratchFile)
	return nil
}

func runRead() error {
	file, err := os.Open(scratchFile)
	if err != nil {
		return err
	}
	defer file.Close()

	ring, err := mapRing(8)
	if err != nil {
		return describeSetupError(err)
	}
	defer ring.close()

	buf := make([]byte, 4096)
	n, err := ring.readAt(file.Fd(), buf, 0)
	if err != nil {
		return err
	}

	fmt.Printf("read %d bytes through IORING_OP_READV from %s:\n%s", n, scratchFile, string(buf[:n]))
	return nil
}

func setupRing(entries uint32) (int, ioUringParams, error) {
	params := ioUringParams{}
	fd, _, errno := syscall.RawSyscall(sysIOUringSetup, uintptr(entries), uintptr(unsafe.Pointer(&params)), 0)
	if errno != 0 {
		return -1, params, errno
	}
	return int(fd), params, nil
}

func mapRing(entries uint32) (*ringMapping, error) {
	fd, params, err := setupRing(entries)
	if err != nil {
		return nil, err
	}

	ring := &ringMapping{fd: fd, params: params}
	if err := ring.mmap(); err != nil {
		ring.close()
		return nil, err
	}
	return ring, nil
}

func (r *ringMapping) mmap() error {
	sqSize := int(r.params.SQOff.Array) + int(r.params.SQEntries)*4
	cqSize := int(r.params.CQOff.CQEs) + int(r.params.CQEntries)*cqeSize

	var err error
	if r.params.Features&ioUringFeatSingleMmap != 0 {
		sharedSize := maxInt(sqSize, cqSize)
		r.sqRing, err = syscall.Mmap(r.fd, ioUringOffSQRing, sharedSize, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED|syscall.MAP_POPULATE)
		if err != nil {
			return fmt.Errorf("mmap shared SQ/CQ ring: %w", err)
		}
		r.cqRing = r.sqRing
	} else {
		r.sqRing, err = syscall.Mmap(r.fd, ioUringOffSQRing, sqSize, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED|syscall.MAP_POPULATE)
		if err != nil {
			return fmt.Errorf("mmap SQ ring: %w", err)
		}
		r.cqRing, err = syscall.Mmap(r.fd, ioUringOffCQRing, cqSize, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED|syscall.MAP_POPULATE)
		if err != nil {
			return fmt.Errorf("mmap CQ ring: %w", err)
		}
	}

	r.sqes, err = syscall.Mmap(r.fd, ioUringOffSQEs, int(r.params.SQEntries)*sqeSize, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED|syscall.MAP_POPULATE)
	if err != nil {
		return fmt.Errorf("mmap SQEs: %w", err)
	}
	return nil
}

func (r *ringMapping) close() {
	if r.sqes != nil {
		syscall.Munmap(r.sqes)
	}
	if r.cqRing != nil && len(r.cqRing) > 0 && &r.cqRing[0] != &r.sqRing[0] {
		syscall.Munmap(r.cqRing)
	}
	if r.sqRing != nil {
		syscall.Munmap(r.sqRing)
	}
	if r.fd >= 0 {
		syscall.Close(r.fd)
	}
}

func (r *ringMapping) submitNOP(userData uint64) (ioUringCQE, error) {
	sqe := ioUringSQE{
		Opcode:   ioUringOpNOP,
		UserData: userData,
	}
	return r.submitAndWait(sqe)
}

func (r *ringMapping) writeAt(fd uintptr, data []byte, offset uint64) (int, error) {
	iov := syscall.Iovec{Base: &data[0], Len: uint64(len(data))}
	sqe := ioUringSQE{
		Opcode:   ioUringOpWriteV,
		FD:       int32(fd),
		Off:      offset,
		Addr:     uint64(uintptr(unsafe.Pointer(&iov))),
		Len:      1,
		UserData: 0x7772697465,
	}
	cqe, err := r.submitAndWait(sqe)
	if err != nil {
		return 0, err
	}
	if cqe.Res < 0 {
		return 0, syscall.Errno(-cqe.Res)
	}
	return int(cqe.Res), nil
}

func (r *ringMapping) readAt(fd uintptr, buf []byte, offset uint64) (int, error) {
	iov := syscall.Iovec{Base: &buf[0], Len: uint64(len(buf))}
	sqe := ioUringSQE{
		Opcode:   ioUringOpReadV,
		FD:       int32(fd),
		Off:      offset,
		Addr:     uint64(uintptr(unsafe.Pointer(&iov))),
		Len:      1,
		UserData: 0x72656164,
	}
	cqe, err := r.submitAndWait(sqe)
	if err != nil {
		return 0, err
	}
	if cqe.Res < 0 {
		return 0, syscall.Errno(-cqe.Res)
	}
	return int(cqe.Res), nil
}

func (r *ringMapping) submitAndWait(sqe ioUringSQE) (ioUringCQE, error) {
	sqHead := loadUint32(r.sqRing, r.params.SQOff.Head)
	sqTail := loadUint32(r.sqRing, r.params.SQOff.Tail)
	sqMask := loadUint32(r.sqRing, r.params.SQOff.RingMask)
	sqEntries := loadUint32(r.sqRing, r.params.SQOff.RingEntries)
	if sqTail-sqHead >= sqEntries {
		return ioUringCQE{}, errors.New("submission queue is full")
	}

	slot := sqTail & sqMask
	*sqeAt(r.sqes, slot) = sqe

	arrayOffset := r.params.SQOff.Array + slot*4
	storeUint32(r.sqRing, arrayOffset, slot)
	storeUint32(r.sqRing, r.params.SQOff.Tail, sqTail+1)

	submitted, _, errno := syscall.RawSyscall6(sysIOUringEnter, uintptr(r.fd), 1, 1, ioUringEnterGetEvents, 0, 0)
	if errno != 0 {
		return ioUringCQE{}, errno
	}
	if submitted != 1 {
		return ioUringCQE{}, fmt.Errorf("submitted=%d, want 1", submitted)
	}
	return r.waitCompletion()
}

func (r *ringMapping) waitCompletion() (ioUringCQE, error) {
	cqHead := loadUint32(r.cqRing, r.params.CQOff.Head)
	cqTail := loadUint32(r.cqRing, r.params.CQOff.Tail)
	if cqHead == cqTail {
		return ioUringCQE{}, errors.New("completion queue is empty")
	}

	cqMask := loadUint32(r.cqRing, r.params.CQOff.RingMask)
	cqe := *cqeAt(r.cqRing, r.params.CQOff.CQEs, cqHead&cqMask)
	storeUint32(r.cqRing, r.params.CQOff.Head, cqHead+1)
	return cqe, nil
}

func sqeAt(sqes []byte, index uint32) *ioUringSQE {
	offset := uintptr(index) * sqeSize
	return (*ioUringSQE)(unsafe.Pointer(&sqes[offset]))
}

func cqeAt(cqRing []byte, base uint32, index uint32) *ioUringCQE {
	offset := uintptr(base) + uintptr(index)*cqeSize
	return (*ioUringCQE)(unsafe.Pointer(&cqRing[offset]))
}

func loadUint32(buf []byte, offset uint32) uint32 {
	return *(*uint32)(unsafe.Pointer(&buf[offset]))
}

func storeUint32(buf []byte, offset uint32, value uint32) {
	*(*uint32)(unsafe.Pointer(&buf[offset])) = value
}

func describeSetupError(err error) error {
	if errors.Is(err, syscall.EPERM) {
		return fmt.Errorf("%w: io_uring is blocked by kernel policy, seccomp, or container restrictions", err)
	}
	if errors.Is(err, syscall.ENOSYS) {
		return fmt.Errorf("%w: kernel does not implement io_uring", err)
	}
	return err
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
