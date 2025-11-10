package ipc

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
)

type shmHeader struct {
	Futex     uint32
	Timestamp uint64
}

// ShmHandler is a shared memory handler for inter-process communication
type ShmHandler struct {
	file                *os.File   // File descriptor for shared memory
	data                []byte     // Pointer to shared memory data
	header              *shmHeader // Pointer to header in shared memory
	size                int        // Size of shared memory
	mode                int        // 0 for reader, 1 for writer
	readerLastTimestamp uint64     // Timestamp of last received packet
}

const (
	modeReader = iota
	modeWriter
	shmFilePrefix = "gsw-service-"
	shmHeaderSize = int(unsafe.Sizeof(shmHeader{}))
)

// NewShmHandler creates a shared memory handler for inter-process communication
func NewShmHandler(identifier string, usableSize int, isWriter bool, shmDir string) (*ShmHandler, error) {
	handler := &ShmHandler{
		size: usableSize + shmHeaderSize, // Add space for header
		mode: modeReader,
	}

	filename := filepath.Join(shmDir, fmt.Sprintf("%s%s", shmFilePrefix, identifier))

	if isWriter {
		handler.mode = modeWriter
		file, err := os.Create(filename)
		if err != nil {
			return nil, fmt.Errorf("failed to create file: %v", err)
		}

		err = file.Truncate(int64(handler.size))
		if err != nil {
			return nil, fmt.Errorf("failed to truncate file: %v", err)
		}
		handler.file = file

		data, err := syscall.Mmap(int(file.Fd()), 0, handler.size, syscall.PROT_WRITE, syscall.MAP_SHARED)
		if err != nil {
			return nil, fmt.Errorf("failed to memory map file: %v", err)
		}
		handler.data = data
	} else {
		file, err := os.OpenFile(filename, os.O_RDWR, 0666)
		if err != nil {
			return nil, fmt.Errorf("failed to open file: %v", err)
		}

		handler.file = file

		data, err := syscall.Mmap(int(file.Fd()), 0, handler.size, syscall.PROT_READ, syscall.MAP_SHARED)
		if err != nil {
			return nil, fmt.Errorf("failed to memory map file: %v", err)
		}

		handler.readerLastTimestamp = uint64(time.Now().UnixNano()) // stream packets after reader start
		handler.data = data
	}

	handler.header = (*shmHeader)(unsafe.Pointer(&handler.data[0]))

	return handler, nil
}

// CreateShmReader creates a shared memory reader for inter-process communication
func CreateShmReader(identifier string, shmDir string) (*ShmHandler, error) {
	fileinfo, err := os.Stat(filepath.Join(shmDir, fmt.Sprintf("%s%s", shmFilePrefix, identifier)))
	if err != nil {
		return nil, fmt.Errorf("error getting shm file info: %v", err)
	}
	filesize := int(fileinfo.Size()) // TODO fix unsafe int64 conversion
	return NewShmHandler(identifier, filesize-shmHeaderSize, false, shmDir)
}

// Cleanup cleans up the shared memory handler and removes the shared memory file
func (handler *ShmHandler) Cleanup() {
	if handler.data != nil {
		if err := syscall.Munmap(handler.data); err != nil {
			fmt.Printf("failed to unmap memory: %v\n", err)
		}
		handler.data = nil
	}
	if handler.file != nil {
		if err := handler.file.Close(); err != nil {
			fmt.Printf("failed to close file: %v\n", err)
		}

		if err := os.Remove(handler.file.Name()); err != nil {
			fmt.Printf("failed to remove file: %v\n", err)
		} else {
			fmt.Printf("Removed file: %s\n", handler.file.Name())
		}

		handler.file = nil
	}
}

// Write writes data to shared memory
func (handler *ShmHandler) Write(data []byte) error {
	if handler.mode != modeWriter {
		return fmt.Errorf("handler is in reader mode")
	}
	if len(data) > handler.size-shmHeaderSize {
		return fmt.Errorf("data size exceeds shared memory size")
	}

	copy(handler.data[shmHeaderSize:len(data)+shmHeaderSize], data)
	handler.header.Timestamp = uint64(time.Now().UnixNano())

	if err := futexWake(unsafe.Pointer(&handler.header.Futex)); err != nil {
		return err
	}
	return nil
}

type ShmReaderPacket struct {
	header shmHeader
	data   []byte
}

// ReceiveTimestamp returns the unix timestamp when the packet was received
// (nanoseconds since epoch).
func (p *ShmReaderPacket) ReceiveTimestamp() uint64 {
	return p.header.Timestamp
}

func (p *ShmReaderPacket) Data() []byte {
	return p.data
}

// wait sleeps the thread until an update to SHM
func (handler *ShmHandler) wait() error {
	return futexWait(unsafe.Pointer(&handler.header.Futex))
}

// Read the current packet in shared memory.
func (handler *ShmHandler) Read() (ReaderPacket, error) {
	if handler.mode != modeReader {
		return nil, fmt.Errorf("handler is in writer mode")
	}

	for {
		err := handler.wait()
		if err != nil {
			return nil, fmt.Errorf("waiting for packet: %w", err)
		}
		packet := ShmReaderPacket{
			header: *handler.header,
			data:   make([]byte, handler.size-shmHeaderSize),
		}
		copy(packet.data, handler.data[shmHeaderSize:handler.size])

		if packet.header.Timestamp <= handler.readerLastTimestamp {
			continue
		}

		handler.readerLastTimestamp = packet.header.Timestamp

		return &packet, nil
	}
}
