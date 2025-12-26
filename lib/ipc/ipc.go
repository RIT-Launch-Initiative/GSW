package ipc

import "context"

// Writer is an interface for sending data across processes
type Writer interface {
	Write(data []byte) error
	Cleanup()
}

// ReaderMessage represents data read from IPC.
type ReaderMessage interface {
	Data() []byte
}

// Reader implements a blocking interface for reading from IPC.
// This is not thread safe.
type Reader interface {
	Read(ctx context.Context) (ReaderMessage, error)
	Cleanup()
}
