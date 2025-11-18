package ipc

import (
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

type shmFileHeader struct {
	futex uint32
}

type shmMessageHeader struct {
	timestamp   uint64
	targetFutex uint32
}

// ShmHandler is a shared memory handler for inter-process communication
type ShmHandler struct {
	file            *os.File       // File descriptor for shared memory
	data            []byte         // Pointer to shared memory data
	header          *shmFileHeader // Pointer to header in shared memory
	messageSize     int            // size of an individual message, including the header
	size            int            // Size of shared memory
	mode            handlerMode    // handler mode: reader or writer
	readerLastFutex uint32         // Last futex word value
}

type handlerMode int

const (
	handlerModeReader handlerMode = iota
	handlerModeWriter
	shmFilePrefix        = "gsw-service-"
	shmFileHeaderSize    = int(unsafe.Sizeof(shmFileHeader{}))
	shmMessageHeaderSize = int(unsafe.Sizeof(shmMessageHeader{}))
	ringSize             = 256
)

// NewShmHandler creates a shared memory handler for inter-process communication
func NewShmHandler(identifier string, telemetryPacketSize int, isWriter bool, shmDir string) (*ShmHandler, error) {
	messageSize := telemetryPacketSize + shmMessageHeaderSize
	handler := &ShmHandler{
		messageSize: messageSize,
		size:        (messageSize * ringSize) + shmFileHeaderSize,
		mode:        handlerModeReader,
	}

	filename := filepath.Join(shmDir, fmt.Sprintf("%s%s", shmFilePrefix, identifier))

	if isWriter {
		handler.mode = handlerModeWriter
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

		handler.data = data
	}

	handler.header = (*shmFileHeader)(unsafe.Pointer(&handler.data[0]))
	handler.readerLastFutex = atomic.LoadUint32(&handler.header.futex)

	return handler, nil
}

// CreateShmReader creates a shared memory reader for inter-process communication
func CreateShmReader(identifier string, shmDir string) (*ShmHandler, error) {
	fileinfo, err := os.Stat(filepath.Join(shmDir, fmt.Sprintf("%s%s", shmFilePrefix, identifier)))
	if err != nil {
		return nil, fmt.Errorf("error getting shm file info: %v", err)
	}
	filesize := int(fileinfo.Size()) // TODO: fix unsafe int64 conversion

	// TODO(mia): guessing packet sizes here is a little suboptimal
	packetSize := ((filesize - shmFileHeaderSize) / ringSize) - shmMessageHeaderSize
	return NewShmHandler(identifier, packetSize, false, shmDir)
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

		if handler.mode == handlerModeWriter {
			if err := os.Remove(handler.file.Name()); err != nil {
				fmt.Printf("failed to remove file: %v\n", err)
			} else {
				fmt.Printf("Removed file: %s\n", handler.file.Name())
			}
		}

		handler.file = nil
	}
}

// Write sends a message to shared memory
func (handler *ShmHandler) Write(data []byte) error {
	if handler.mode != handlerModeWriter {
		return fmt.Errorf("handler is in reader mode")
	}
	if len(data) > (handler.messageSize - shmMessageHeaderSize) {
		return fmt.Errorf("data size exceeds allocated message size")
	}

	targetFutex := atomic.LoadUint32(&handler.header.futex) + 1
	messagePosition := shmFileHeaderSize + int(targetFutex%ringSize)*handler.messageSize

	messageHeader := (*shmMessageHeader)(unsafe.Pointer(&handler.data[messagePosition]))

	dataPosition := messagePosition + shmMessageHeaderSize
	copy(handler.data[dataPosition:], data)

	*messageHeader = shmMessageHeader{
		timestamp:   uint64(time.Now().UnixNano()),
		targetFutex: targetFutex,
	}

	atomic.StoreUint32(&handler.header.futex, targetFutex)
	if err := futexWake(unsafe.Pointer(&handler.header.futex)); err != nil {
		return err
	}
	return nil
}

// ShmReaderMessage is a message read by an ShmHandler
type ShmReaderMessage struct {
	timestamp uint64
	futex     uint32
	data      []byte
}

// ReceiveTimestamp returns the unix timestamp when the message was received
// (nanoseconds since epoch).
func (m *ShmReaderMessage) ReceiveTimestamp() uint64 {
	return m.timestamp
}

// ReceiveTimestamp returns the message futex value (an incrementing counter).
// This could be used to estimate message loss.
func (m *ShmReaderMessage) Futex() uint32 {
	return m.futex
}

// Data returns the message data.
func (m *ShmReaderMessage) Data() []byte {
	return m.data
}

// wait sleeps the thread until an update to SHM.
// Only waits if the futex value is not outdated.
func (handler *ShmHandler) wait() error {
	currentFutex := atomic.LoadUint32(&handler.header.futex)
	if handler.readerLastFutex != currentFutex {
		return nil
	}

	return futexWait(unsafe.Pointer(&handler.header.futex), handler.readerLastFutex)
}

// Read the current message in shared memory.
func (handler *ShmHandler) Read() (ReaderMessage, error) {
	if handler.mode != handlerModeReader {
		return nil, fmt.Errorf("handler is in writer mode")
	}

	for {
		err := handler.wait()
		if err != nil {
			return nil, fmt.Errorf("waiting for message: %w", err)
		}

		newMessageFutex := atomic.LoadUint32(&handler.header.futex)

		// HACK-ish(mia): This means that the thread woke superfluously,
		// not sure why why this happens. futex should compare the value
		// before waiting again, so it shouldn't cause an erroneous wait.
		if newMessageFutex <= handler.readerLastFutex {
			continue
		}

		messagePosition := shmFileHeaderSize + int(newMessageFutex%ringSize)*handler.messageSize

		shmData := make([]byte, handler.messageSize)
		copy(shmData, handler.data[messagePosition:])

		messageHeader := (*shmMessageHeader)(unsafe.Pointer(&shmData[0]))
		message := ShmReaderMessage{
			timestamp: messageHeader.timestamp,
			futex:     newMessageFutex,
			data:      shmData[shmMessageHeaderSize:],
		}

		// HACK(mia): if the message header does not match the message we want
		// to be reading, it is not the right message.
		if messageHeader.targetFutex != newMessageFutex {
			continue
		}

		handler.readerLastFutex = newMessageFutex

		return &message, nil
	}
}

// ReadRaw returns a copy of the current packet in SHM.
func (handler *ShmHandler) ReadRaw() ([]byte, error) {
	if handler.mode != handlerModeReader {
		return nil, fmt.Errorf("handler is in writer mode")
	}

	messageFutex := atomic.LoadUint32(&handler.header.futex)

	messagePosition := shmFileHeaderSize + int(messageFutex%ringSize)*handler.messageSize

	shmData := make([]byte, handler.messageSize-shmMessageHeaderSize)
	copy(shmData, handler.data[messagePosition+shmMessageHeaderSize:])

	return shmData, nil
}
