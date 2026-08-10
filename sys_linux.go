package pact

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

var pageSize = uintptr(unix.Getpagesize())

// reserve reserves inaccessible virtual memory.
func reserve(size uintptr) (unsafe.Pointer, error) {
	return unix.MmapPtr(
		-1,
		0,
		nil,
		size,
		unix.PROT_NONE,
		unix.MAP_ANON|unix.MAP_PRIVATE,
	)
}

// commit makes a reserved range readable and writable.
func commit(addr unsafe.Pointer, size uintptr) error {
	b := unsafe.Slice((*byte)(addr), size)
	return unix.Mprotect(b, unix.PROT_READ|unix.PROT_WRITE)
}

// decommit discards the pages backing a range and makes it inaccessible.
func decommit(addr unsafe.Pointer, size uintptr) error {
	b := unsafe.Slice((*byte)(addr), size)
	if err := unix.Madvise(b, unix.MADV_DONTNEED); err != nil {
		return err
	}
	return unix.Mprotect(b, unix.PROT_NONE)
}

// release unmaps a reserved range.
func release(addr unsafe.Pointer, size uintptr) error {
	return unix.MunmapPtr(addr, size)
}
