package ipc

// Writer is an interface for sending data across processes
type Writer interface {
	Write(data []byte) error
	Cleanup()
}

// Reader is an interface for receiving data across processes
type Reader interface {
	Read() ([]byte, error)
	Cleanup()
}
