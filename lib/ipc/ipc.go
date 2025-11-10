package ipc

// Writer is an interface for sending data across processes
type Writer interface {
	Write(data []byte) error
	Cleanup()
}

type ReaderPacket interface {
	Data() []byte
}

// Reader implements a blocking interface for reading from an IPC.
// This is not thread safe.
type Reader interface {
	Read() (ReaderPacket, error)
	Cleanup()
}
