package pact

import "unsafe"

var _ PoolAllocator = (*Pool)(nil)

// freeBlock is the link stored in a free block.
type freeBlock struct {
	next *freeBlock
}

// Pool allocates fixed-size raw blocks from a lazily committed virtual memory region.
// A block may be overlaid with a type when its size and alignment are sufficient.
type Pool struct {
	reg       *region
	free      *freeBlock
	bitmap    []byte
	off       uintptr
	blockSize uintptr
	capacity  uintptr
	allocated uintptr
}

// NewPool creates a Pool with capacity blocks of at least size bytes.
// align must be a power of two when it is greater than one.
func NewPool(size, align, capacity uintptr) (*Pool, error) {
	var zero freeBlock
	if capacity == 0 {
		return nil, ErrInvalidCapacity
	}

	blockSize := max(size, unsafe.Sizeof(zero))
	if align > 1 {
		if align&(align-1) != 0 {
			return nil, ErrInvalidAlignment
		}
		blockSize = alignUp(blockSize, align)
	}

	if blockSize == maxUintptr || capacity > maxUintptr/blockSize {
		return nil, ErrOutOfMemory
	}

	usable := blockSize * capacity
	if usable > maxUintptr-(pageSize-1) {
		return nil, ErrOutOfMemory
	}

	bitmapSize := capacity / 8
	if capacity%8 != 0 {
		bitmapSize++
	}

	if bitmapSize > maxIntValue {
		return nil, ErrOutOfMemory
	}

	reg, err := newRegion(usable)
	if err != nil {
		return nil, err
	}
	return &Pool{
		reg:       reg,
		bitmap:    make([]byte, int(bitmapSize)),
		blockSize: blockSize,
		capacity:  capacity,
	}, nil
}

// Alloc returns a block from the Pool.
func (p *Pool) Alloc() (unsafe.Pointer, error) {
	if p.reg == nil {
		return nil, ErrReleased
	}

	if p.free != nil {
		obj := p.free
		ptr := unsafe.Pointer(obj)
		p.free = obj.next
		obj.next = nil
		p.markInUse((uintptr(ptr) - uintptr(p.reg.addr)) / p.blockSize)
		return ptr, nil
	}

	if p.allocated >= p.capacity {
		return nil, ErrOutOfMemory
	}

	addr, off, err := p.reg.address(p.off, p.blockSize, 0)
	if err != nil {
		return nil, err
	}
	p.off = off
	p.allocated++
	p.markInUse((off - p.blockSize) / p.blockSize)
	return addr, nil
}

// Free returns ptr to the Pool.
func (p *Pool) Free(ptr unsafe.Pointer) error {
	if p.reg == nil {
		return ErrReleased
	}

	pointer := uintptr(ptr)
	start := uintptr(p.reg.addr)
	off := pointer - start
	if start > pointer || off >= p.off || off%p.blockSize != 0 {
		return ErrInvalidPointer
	}

	if !p.markFree(off / p.blockSize) {
		return ErrAlreadyFree
	}
	block := (*freeBlock)(ptr)
	block.next = p.free
	p.free = block
	return nil
}

// Release releases the Pool's virtual memory reservation.
func (p *Pool) Release() error {
	if p.reg == nil {
		return ErrReleased
	}

	if err := p.reg.release(); err != nil {
		return err
	}
	p.reg = nil
	p.free = nil
	p.bitmap = nil
	p.off = 0
	p.blockSize = 0
	p.capacity = 0
	p.allocated = 0
	return nil
}

// markInUse marks a block as allocated.
func (p *Pool) markInUse(index uintptr) {
	byteIndex := index >> 3
	bit := byte(1 << (index & 7))
	p.bitmap[byteIndex] |= bit
}

// markFree returns whether the block was allocated and marks it free.
func (p *Pool) markFree(index uintptr) bool {
	byteIndex := index >> 3
	bit := byte(1 << (index & 7))
	if p.bitmap[byteIndex]&bit == 0 {
		return false
	}
	p.bitmap[byteIndex] &^= bit
	return true
}
