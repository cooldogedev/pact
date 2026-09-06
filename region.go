package pact

import "unsafe"

// region owns a virtual memory reservation and commits pages as needed.
type region struct {
	addr      unsafe.Pointer
	reserved  uintptr
	committed uintptr
}

// newRegion reserves at least size bytes without committing them.
func newRegion(size uintptr) (*region, error) {
	if size == 0 {
		return nil, ErrInvalidReserveSize
	}

	reserved := alignUp(size, pageSize)
	if reserved == maxUintptr {
		return nil, ErrOutOfMemory
	}

	addr, err := reserve(reserved)
	if err != nil {
		return nil, err
	}
	return &region{
		addr:     addr,
		reserved: reserved,
	}, nil
}

// address returns an aligned address and the offset after the allocation.
// It commits pages as needed.
func (r *region) address(offset, size, align uintptr) (unsafe.Pointer, uintptr, error) {
	if size == 0 {
		return nil, 0, ErrInvalidSize
	}

	if align > pageSize {
		return nil, 0, ErrInvalidAlignment
	}

	if align > 1 {
		offset = alignUp(offset, align)
	}

	end := offset + size
	if end < offset {
		return nil, 0, ErrOutOfMemory
	}

	if end > r.committed {
		if err := r.grow(end); err != nil {
			return nil, 0, err
		}
	}
	return unsafe.Add(r.addr, offset), end, nil
}

// trim decommits pages after keep while retaining the reservation.
func (r *region) trim(keep uintptr) error {
	end := alignUp(keep, pageSize)
	if end >= r.committed {
		return nil
	}

	if err := decommit(unsafe.Add(r.addr, end), r.committed-end); err != nil {
		return err
	}
	r.committed = end
	return nil
}

// release releases the reservation and clears the region.
func (r *region) release() error {
	if err := release(r.addr, r.reserved); err != nil {
		return err
	}
	r.addr = nil
	r.reserved = 0
	r.committed = 0
	return nil
}

// grow commits enough pages to cover required.
func (r *region) grow(required uintptr) error {
	end := alignUp(required, pageSize)
	if end > r.reserved {
		return ErrOutOfMemory
	}

	if err := commit(unsafe.Add(r.addr, r.committed), end-r.committed); err != nil {
		return err
	}
	r.committed = end
	return nil
}
