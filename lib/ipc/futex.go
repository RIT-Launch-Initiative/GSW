package ipc

import (
	"fmt"
	"math"
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

// futexWait waits for a futex at an address.
// Internally, futex atomically checks if *addr == val before waiting.
func futexWait(addr unsafe.Pointer, val uint32) error {
	_, errno := futex(addr, _FUTEX_WAIT, val, nil, nil, 0)
	if errno != 0 && errno != syscall.EAGAIN {
		return fmt.Errorf("FUTEX_WAIT: %v", errno)
	}
	return nil
}

// futexWake wakes all threads sleeping at an address
func futexWake(addr unsafe.Pointer) error {
	_, errno := futex(addr, _FUTEX_WAKE, math.MaxInt32, nil, nil, 0)
	if errno != 0 {
		return fmt.Errorf("FUTEX_WAKE: %v", errno)
	}
	return nil
}
