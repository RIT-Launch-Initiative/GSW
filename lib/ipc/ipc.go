package ipc

type Writer interface {
	Write(data []byte) error
	Cleanup()
}

type Reader interface {
	Read() ([]byte, error)
	Cleanup()
}
