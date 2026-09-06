package pact

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var pageSize = uintptr(windows.Getpagesize())

// reserve reserves inaccessible virtual memory.
func reserve(size uintptr) (unsafe.Pointer, error) {
	addr, err := windows.VirtualAlloc(
		0,
		size,
		windows.MEM_RESERVE,
		windows.PAGE_NOACCESS,
	)
	if err != nil {
		return nil, err
	}
	return unsafe.Add(nil, addr), nil
}

// commit makes a reserved range readable and writable.
func commit(addr unsafe.Pointer, size uintptr) error {
	_, err := windows.VirtualAlloc(
		uintptr(addr),
		size,
		windows.MEM_COMMIT,
		windows.PAGE_READWRITE,
	)
	return err
}

// decommit releases a range's physical storage but keeps its reservation.
func decommit(addr unsafe.Pointer, size uintptr) error {
	return windows.VirtualFree(uintptr(addr), size, windows.MEM_DECOMMIT)
}

// release releases a reserved range.
func release(addr unsafe.Pointer, _ uintptr) error {
	return windows.VirtualFree(uintptr(addr), 0, windows.MEM_RELEASE)
}
