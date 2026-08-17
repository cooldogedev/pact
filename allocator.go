package pact

import (
	"errors"
	"unsafe"
)

var (
	ErrOutOfMemory = errors.New("out of memory")

	ErrReleased = errors.New("allocator is released")

	ErrInvalidReserveSize = errors.New("invalid reserve size")

	ErrInvalidCapacity = errors.New("invalid capacity")
	ErrInvalidLength   = errors.New("invalid length")
	ErrInvalidSize     = errors.New("invalid size")

	ErrInvalidAlignment = errors.New("invalid alignment")
	ErrInvalidPointer   = errors.New("invalid pointer")
	ErrAlreadyFree      = errors.New("pointer is already free")
)

const (
	// maxUintptr is the largest value that fits in a uintptr.
	maxUintptr  = ^uintptr(0)
	maxIntValue = maxUintptr >> 1
)

// ByteAllocator allocates raw memory that remains valid until Release.
// Allocations are not freed individually.
// The memory is outside the Go heap and is not scanned by the garbage collector.
type ByteAllocator interface {
	// Alloc allocates size bytes at the requested alignment. align must be zero,
	// one, or a power of two. Zero-size allocations are not supported, and the
	// allocator does not clear memory.
	Alloc(size, align uintptr) (unsafe.Pointer, error)
	// Release releases all memory owned by the allocator.
	Release() error
}

// ObjectAllocator allocates objects of type T and allows them to be reused.
// The objects live outside the Go heap and are not scanned by the garbage collector.
type ObjectAllocator[T any] interface {
	// Alloc returns an object of type T owned by the allocator. Its memory is
	// not initialised by the allocator.
	Alloc() (*T, error)
	// Free returns an object to the allocator for reuse.
	Free(obj *T) error
	// Release releases all memory owned by the allocator.
	Release() error
}

// PoolAllocator allocates fixed-size raw blocks and allows them to be reused.
type PoolAllocator interface {
	// Alloc returns a block of raw memory owned by the allocator.
	Alloc() (unsafe.Pointer, error)
	// Free returns a block to the allocator for reuse.
	Free(ptr unsafe.Pointer) error
	// Release releases all memory owned by the allocator.
	Release() error
}

// Must returns val or panics with err.
func Must[T any](val T, err error) T {
	if err != nil {
		panic(err)
	}
	return val
}

// AllocNew allocates and returns a T-sized, T-aligned block.
// It returns nil for a zero-sized T.
func AllocNew[T any](alloc ByteAllocator) (*T, error) {
	var zero T
	size := unsafe.Sizeof(zero)
	if size == 0 {
		return nil, nil
	}

	ptr, err := alloc.Alloc(size, unsafe.Alignof(zero))
	if err != nil {
		return nil, err
	}
	return (*T)(ptr), nil
}

// AllocSlice allocates capacity elements of T and returns the first length.
// If capacity is omitted, length is used. A zero capacity returns nil.
func AllocSlice[T any](alloc ByteAllocator, length uintptr, capacity ...uintptr) ([]T, error) {
	var c uintptr
	if len(capacity) > 0 {
		c = capacity[0]
	} else {
		c = length
	}

	if length > c {
		return nil, ErrInvalidLength
	}

	if c > maxIntValue {
		return nil, ErrOutOfMemory
	}

	if c == 0 {
		return nil, nil
	}

	var zero T
	size := unsafe.Sizeof(zero)
	if size != 0 && c > maxUintptr/size {
		return nil, ErrOutOfMemory
	}

	ptr, err := alloc.Alloc(size*c, unsafe.Alignof(zero))
	if err != nil {
		return nil, err
	}
	s := unsafe.Slice((*T)(ptr), c)
	return s[:length], nil
}

// AllocString returns a copy of s stored in allocator-owned memory.
// Empty strings are returned without allocating.
func AllocString(alloc ByteAllocator, s string) (string, error) {
	if len(s) == 0 {
		return "", nil
	}

	ptr, err := alloc.Alloc(uintptr(len(s)), 1)
	if err != nil {
		return "", err
	}
	dst := unsafe.Slice((*byte)(ptr), len(s))
	copy(dst, s)
	return unsafe.String((*byte)(ptr), len(s)), nil

}

// alignUp rounds x up to the next multiple of align.
// align must be a non-zero power of two. It returns maxUintptr on overflow.
func alignUp(x, align uintptr) uintptr {
	if x > maxUintptr-(align-1) {
		return maxUintptr
	}
	return (x + align - 1) &^ (align - 1)
}

// alignOffset aligns base+offset and returns the resulting offset from base.
func alignOffset(base, offset, align uintptr) (uintptr, error) {
	if offset > maxUintptr-base {
		return 0, ErrOutOfMemory
	}

	address := alignUp(base+offset, align)
	if address == maxUintptr {
		return 0, ErrOutOfMemory
	}
	return address - base, nil
}
