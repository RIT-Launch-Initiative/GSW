package ipc

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

type IpcShmHandler struct {
	file            *os.File
	data            []byte
	size            int
	mode            int // 0 for reader, 1 for writer
	timestampOffset int // Offset for the timestamp in shared memory
}

const (
	modeReader = iota
	modeWriter
	timestampSize = 8 // Size of timestamp in bytes (8 bytes for int64)
)

func CreateIpcShmHandler(identifier string, size int, isWriter bool) (*IpcShmHandler, error) {
	handler := &IpcShmHandler{
		size:            size + timestampSize, // Add space for timestamp
		mode:            modeReader,
		timestampOffset: size, // Timestamp is stored at the end
	}

	filename := filepath.Join("/dev/shm", fmt.Sprintf("gsw-service-%s", identifier))

	if isWriter {
		handler.mode = modeWriter
		file, err := os.Create(filename)
		if err != nil {
			return nil, fmt.Errorf("Failed to create file: %v", err)
		}
		defer func(file *os.File) {
			err := file.Close()
			if err != nil {
				fmt.Printf("Failed to close file: %v\n", err)
			}
		}(file)

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

func (handler *IpcShmHandler) Read() ([]byte, error) {
	if handler.mode != modeReader {
		return nil, fmt.Errorf("Handler is in writer mode")
	}
	data := make([]byte, handler.size-timestampSize)
	copy(data, handler.data[:len(data)])
	return data, nil
}

func (handler *IpcShmHandler) LastUpdate() time.Time {
	timestamp := binary.BigEndian.Uint64(handler.data[handler.timestampOffset:])
	return time.Unix(0, int64(timestamp))
}
