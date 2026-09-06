package pact

import (
	"errors"
	"testing"
	"unsafe"
)

func newTestPool(t *testing.T, size, align, capacity uintptr) *Pool {
	t.Helper()
	pool, err := NewPool(size, align, capacity)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if pool.reg == nil {
			return
		}

		if err := pool.Release(); err != nil {
			t.Errorf("cleanup release: %v", err)
		}
	})
	return pool
}

func TestNewPoolRejectsInvalidCapacity(t *testing.T) {
	if _, err := NewPool(8, 1, 0); !errors.Is(err, ErrInvalidCapacity) {
		t.Fatalf("NewPool() = %v, want %v", err, ErrInvalidCapacity)
	}
}

func TestNewPoolRejectsAlignmentAbovePageSize(t *testing.T) {
	pool, err := NewPool(8, pageSize*2, 1)
	if pool != nil {
		defer pool.Release()
	}

	if !errors.Is(err, ErrInvalidAlignment) {
		t.Fatalf("NewPool() = %v, want %v", err, ErrInvalidAlignment)
	}
}

func TestNewPoolRejectsInvalidAlignment(t *testing.T) {
	if _, err := NewPool(8, 3, 1); !errors.Is(err, ErrInvalidAlignment) {
		t.Fatalf("NewPool() = %v, want %v", err, ErrInvalidAlignment)
	}
}

func TestNewPoolRejectsBlockSizeOverflow(t *testing.T) {
	if _, err := NewPool(maxUintptr-1, 2, 1); !errors.Is(err, ErrOutOfMemory) {
		t.Fatalf("NewPool() = %v, want %v", err, ErrOutOfMemory)
	}
}

func TestPoolMinimumBlockFitsFreeList(t *testing.T) {
	pool := newTestPool(t, 1, 1, 1)
	ptr, err := pool.Alloc()
	if err != nil {
		t.Fatal(err)
	}

	if err := pool.Free(ptr); err != nil {
		t.Fatalf("Free() after minimum-size allocation = %v", err)
	}
}

func TestPoolAllocationsAreAlignedAndIndependent(t *testing.T) {
	pool := newTestPool(t, 1, 16, 2)
	first, err := pool.Alloc()
	if err != nil {
		t.Fatal(err)
	}

	second, err := pool.Alloc()
	if err != nil {
		t.Fatal(err)
	}

	if uintptr(first)%16 != 0 || uintptr(second)%16 != 0 {
		t.Fatalf("blocks %p and %p are not aligned", first, second)
	}

	*(*byte)(first) = 0xaa
	*(*byte)(second) = 0x55
	if *(*byte)(first) != 0xaa || *(*byte)(second) != 0x55 {
		t.Fatal("pool blocks overlap")
	}
}

func TestPoolBlocksCanReferenceEachOther(t *testing.T) {
	type node struct {
		value uint64
		next  *node
	}

	pool := newTestPool(t, unsafe.Sizeof(node{}), unsafe.Alignof(node{}), 2)
	first, err := pool.Alloc()
	if err != nil {
		t.Fatal(err)
	}

	second, err := pool.Alloc()
	if err != nil {
		t.Fatal(err)
	}

	a, b := (*node)(first), (*node)(second)
	a.value, b.value = 1, 2
	a.next = b
	if a.next != b || a.next.value != 2 {
		t.Fatalf("pool object link = %p, want %p with value 2", a.next, b)
	}
}

func TestPoolReturnsOutOfMemoryWhenFull(t *testing.T) {
	pool := newTestPool(t, 8, 1, 1)
	if _, err := pool.Alloc(); err != nil {
		t.Fatal(err)
	}

	if _, err := pool.Alloc(); !errors.Is(err, ErrOutOfMemory) {
		t.Fatalf("second Alloc() = %v, want %v", err, ErrOutOfMemory)
	}
}

func TestPoolReusesFreedBlocksInLIFOOrder(t *testing.T) {
	pool := newTestPool(t, 8, 1, 2)
	first, err := pool.Alloc()
	if err != nil {
		t.Fatal(err)
	}

	second, err := pool.Alloc()
	if err != nil {
		t.Fatal(err)
	}

	if err := pool.Free(first); err != nil {
		t.Fatal(err)
	}

	if err := pool.Free(second); err != nil {
		t.Fatal(err)
	}

	reused, err := pool.Alloc()
	if err != nil {
		t.Fatal(err)
	}

	if reused != second {
		t.Fatalf("first reused block = %p, want %p", reused, second)
	}

	reused, err = pool.Alloc()
	if err != nil {
		t.Fatal(err)
	}

	if reused != first {
		t.Fatalf("second reused block = %p, want %p", reused, first)
	}
}

func TestPoolRejectsDoubleFree(t *testing.T) {
	pool := newTestPool(t, 8, 1, 1)
	ptr, err := pool.Alloc()
	if err != nil {
		t.Fatal(err)
	}

	if err := pool.Free(ptr); err != nil {
		t.Fatal(err)
	}

	if err := pool.Free(ptr); !errors.Is(err, ErrAlreadyFree) {
		t.Fatalf("second Free() = %v, want %v", err, ErrAlreadyFree)
	}
}

func TestPoolFreeRejectsInvalidPointer(t *testing.T) {
	pool := newTestPool(t, 8, 1, 2)
	allocated, err := pool.Alloc()
	if err != nil {
		t.Fatal(err)
	}

	other := newTestPool(t, 8, 1, 1)
	foreign, err := other.Alloc()
	if err != nil {
		t.Fatal(err)
	}

	for _, ptr := range []unsafe.Pointer{
		nil,
		unsafe.Add(allocated, 1),
		foreign,
		unsafe.Add(allocated, pool.blockSize),
	} {
		if err := pool.Free(ptr); !errors.Is(err, ErrInvalidPointer) {
			t.Errorf("Free(%p) = %v, want %v", ptr, err, ErrInvalidPointer)
		}
	}
}

func TestPoolRejectsAllocationAfterRelease(t *testing.T) {
	pool := newTestPool(t, 8, 1, 1)
	if err := pool.Release(); err != nil {
		t.Fatal(err)
	}

	if _, err := pool.Alloc(); !errors.Is(err, ErrReleased) {
		t.Fatalf("Alloc() after release = %v, want %v", err, ErrReleased)
	}
}

func TestPoolRejectsSecondRelease(t *testing.T) {
	pool := newTestPool(t, 8, 1, 1)
	if err := pool.Release(); err != nil {
		t.Fatal(err)
	}

	if err := pool.Release(); !errors.Is(err, ErrReleased) {
		t.Fatalf("second Release() = %v, want %v", err, ErrReleased)
	}
}
