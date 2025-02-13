package ipc

import (
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// IpcShmHandler is a shared memory handler for inter-process communication
type IpcShmHandler struct {
	file            *os.File // File descriptor for shared memory
	data            []byte   // Pointer to shared memory data
	size            int      // Size of shared memory
	mode            int      // 0 for reader, 1 for writer
	timestampOffset int      // Offset for the timestamp in shared memory
}

const (
	modeReader = iota
	modeWriter
	timestampSize = 8 // Size of timestamp in bytes (8 bytes for int64)
	shmFilePrefix = "gsw-service-"
)

var shmDir = flag.String("shm", "/dev/shm", "directory to use for shared memory")

// CreateIpcShmHandler creates a shared memory handler for inter-process communication
func CreateIpcShmHandler(identifier string, size int, isWriter bool) (*IpcShmHandler, error) {
	handler := &IpcShmHandler{
		size:            size + timestampSize, // Add space for timestamp
		mode:            modeReader,
		timestampOffset: size, // Timestamp is stored at the end
	}

	flag.Parse()
	filename := filepath.Join(*shmDir, fmt.Sprintf("%s%s", shmFilePrefix, identifier))

	if isWriter {
		handler.mode = modeWriter
		file, err := os.Create(filename)
		if err != nil {
			return nil, fmt.Errorf("Failed to create file: %v", err)
		}
		/* maybe we don't need this
		defer func(file *os.File) {
			if file != nil {
				err := file.Close()
				if err != nil {
					fmt.Printf("Failed to close file: %v\n", err)
				}
			}
		}(file)
		*/

		err = file.Truncate(int64(handler.size))
		if err != nil {
			return nil, fmt.Errorf("Failed to truncate file: %v", err)
		}
		handler.file = file

		data, err := syscall.Mmap(int(file.Fd()), 0, handler.size, syscall.PROT_WRITE, syscall.MAP_SHARED)
		if err != nil {
			return nil, fmt.Errorf("Failed to memory map file: %v", err)
		}
		handler.data = data
	} else {
		file, err := os.OpenFile(filename, os.O_RDWR, 0666)
		if err != nil {
			return nil, fmt.Errorf("Failed to open file: %v", err)
		}

		handler.file = file

		data, err := syscall.Mmap(int(file.Fd()), 0, handler.size, syscall.PROT_READ, syscall.MAP_SHARED)
		if err != nil {
			return nil, fmt.Errorf("Failed to memory map file: %v", err)
		}

		handler.data = data
	}

	return handler, nil
}

// CreateIpcShmReader creates a shared memory reader for inter-process communication
func CreateIpcShmReader(identifier string) (*IpcShmHandler, error) {
	flag.Parse()
	fileinfo, err := os.Stat(filepath.Join(*shmDir, fmt.Sprintf("%s%s", shmFilePrefix, identifier)))
	if err != nil {
		return nil, fmt.Errorf("Error getting shm file info: %v", err)
	}
	filesize := int(fileinfo.Size()) // TODO fix unsafe int64 conversion
	return CreateIpcShmHandler(identifier, filesize, false)
}

// Cleanup cleans up the shared memory handler and removes the shared memory file
func (handler *IpcShmHandler) Cleanup() {
	if handler.data != nil {
		if err := syscall.Munmap(handler.data); err != nil {
			fmt.Printf("Failed to unmap memory: %v\n", err)
		}
		handler.data = nil
	}
	if handler.file != nil {
		if err := handler.file.Close(); err != nil {
			fmt.Printf("Failed to close file: %v\n", err)
		}

		if err := os.Remove(handler.file.Name()); err != nil {
			fmt.Printf("Failed to remove file: %v\n", err)
		} else {
			fmt.Printf("Removed file: %s\n", handler.file.Name())
		}

		handler.file = nil
	}
}

// Write writes data to shared memory
func (handler *IpcShmHandler) Write(data []byte) error {
	if handler.mode != modeWriter {
		return fmt.Errorf("Handler is in reader mode")
	}
	if len(data) > handler.size-timestampSize {
		return fmt.Errorf("Data size exceeds shared memory size")
	}

	copy(handler.data[:len(data)], data)
	binary.BigEndian.PutUint64(handler.data[handler.timestampOffset:], uint64(time.Now().UnixNano()))
	return nil
}

// Read reads data from shared memory
func (handler *IpcShmHandler) Read() ([]byte, error) {
	if handler.mode != modeReader {
		return nil, fmt.Errorf("Handler is in writer mode")
	}
	data := make([]byte, handler.size-timestampSize)
	copy(data, handler.data[:len(data)])
	return data, nil
}

// ReadNoTimestamp reads data from shared memory without the timestamp
func (handler *IpcShmHandler) ReadNoTimestamp() ([]byte, error) {
	if handler.mode != modeReader {
		return nil, fmt.Errorf("Handler is in writer mode")
	}
	data := make([]byte, handler.size-2*timestampSize)
	copy(data, handler.data[:len(data)])
	return data, nil
}

// LastUpdate returns the last update time of the shared memory
func (handler *IpcShmHandler) LastUpdate() time.Time {
	timestamp := binary.BigEndian.Uint64(handler.data[handler.timestampOffset:])
	return time.Unix(0, int64(timestamp))
}
