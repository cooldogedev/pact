package pact

import "unsafe"

var _ ObjectAllocator[any] = (*Slab[any])(nil)

// Slab allocates objects of type T from a fixed-size Pool.
type Slab[T any] struct {
	pool *Pool
}

// NewSlab creates a Slab with room for capacity objects.
func NewSlab[T any](capacity uintptr) (*Slab[T], error) {
	var zero T
	pool, err := NewPool(unsafe.Sizeof(zero), unsafe.Alignof(zero), capacity)
	if err != nil {
		return nil, err
	}
	return &Slab[T]{pool: pool}, nil
}

// Alloc returns an object from the Slab.
func (s *Slab[T]) Alloc() (*T, error) {
	ptr, err := s.pool.Alloc()
	if err != nil {
		return nil, err
	}
	return (*T)(ptr), nil
}

// Free returns obj to the Slab.
func (s *Slab[T]) Free(obj *T) error {
	return s.pool.Free(unsafe.Pointer(obj))
}

// Release releases the Slab's memory.
func (s *Slab[T]) Release() error {
	return s.pool.Release()
}
