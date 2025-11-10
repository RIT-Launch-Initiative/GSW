package ipc

import (
	"fmt"
	"syscall"
	"unsafe"
)

const (
	_FUTEX_WAIT = 0
	_FUTEX_WAKE = 1
)

// futex syscall wrapper
func futex(addr unsafe.Pointer, op int, val uint32, timeout unsafe.Pointer, addr2 unsafe.Pointer, val3 uint32) (int, syscall.Errno) {
	r1, _, errno := syscall.Syscall6(syscall.SYS_FUTEX, uintptr(addr), uintptr(op), uintptr(val), uintptr(timeout), uintptr(addr2), uintptr(val3))
	return int(r1), errno
}

// futexWait wait for a futex at an address
func futexWait(addr unsafe.Pointer) error {
	_, errno := futex(addr, _FUTEX_WAIT, 0, nil, nil, 0)
	if errno != 0 {
		return fmt.Errorf("FUTEX_WAIT: %v", errno)
	}
	return nil
}

// futexWake wake an address
func futexWake(addr unsafe.Pointer) error {
	_, errno := futex(addr, _FUTEX_WAKE, 0, nil, nil, 0)
	if errno != 0 {
		return fmt.Errorf("FUTEX_WAKE: %v", errno)
	}
	return nil
}
