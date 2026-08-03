package pact

import "unsafe"

var _ ByteAllocator = (*Arena)(nil)

// Arena is a bump allocator backed by a lazily committed virtual memory region.
type Arena struct {
	reg *region
	off uintptr
}

// NewArena reserves size bytes for an Arena.
func NewArena(size uintptr) (*Arena, error) {
	reg, err := newRegion(size)
	if err != nil {
		return nil, err
	}
	return &Arena{reg: reg}, nil
}

// Alloc allocates size bytes at the requested alignment.
// align must be zero, one, or a power of two.
func (a *Arena) Alloc(size, align uintptr) (unsafe.Pointer, error) {
	if a.reg == nil {
		return nil, ErrReleased
	}

	if align > 1 && align&(align-1) != 0 {
		return nil, ErrInvalidAlignment
	}

	addr, next, err := a.reg.address(a.off, size, align)
	if err != nil {
		return nil, err
	}
	a.off = next
	return addr, nil
}

// Reset makes all previous allocations available again without clearing memory.
func (a *Arena) Reset() error {
	if a.reg == nil {
		return ErrReleased
	}
	a.off = 0
	return nil
}

// Trim decommits pages beyond the current allocation offset.
func (a *Arena) Trim() error {
	if a.reg == nil {
		return ErrReleased
	}
	return a.reg.trim(a.off)
}

// Release releases the Arena's virtual memory reservation.
func (a *Arena) Release() error {
	if a.reg == nil {
		return ErrReleased
	}

	if err := a.reg.release(); err != nil {
		return err
	}
	a.reg = nil
	return nil
}
