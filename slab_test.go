package pact

import (
	"errors"
	"testing"
	"unsafe"
)

// slabValue leaves payload after the word used by Pool's free-list link.
type slabValue struct {
	link    uintptr
	payload uint64
}

func newTestSlab(t *testing.T, capacity uintptr) *Slab[slabValue] {
	t.Helper()
	slab, err := NewSlab[slabValue](capacity)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if slab.pool == nil || slab.pool.reg == nil {
			return
		}

		if err := slab.Release(); err != nil {
			t.Errorf("cleanup release: %v", err)
		}
	})
	return slab
}

func TestNewSlabRejectsZeroCapacity(t *testing.T) {
	if _, err := NewSlab[slabValue](0); !errors.Is(err, ErrInvalidCapacity) {
		t.Fatalf("NewSlab() = %v, want %v", err, ErrInvalidCapacity)
	}
}

func TestSlabReusesAlignedObjectAndPreservesPayload(t *testing.T) {
	slab := newTestSlab(t, 1)
	first, err := slab.Alloc()
	if err != nil {
		t.Fatal(err)
	}

	first.payload = 42
	if uintptr(unsafe.Pointer(first))%unsafe.Alignof(slabValue{}) != 0 {
		t.Fatalf("object %p is not aligned for slabValue", first)
	}

	if err := slab.Free(first); err != nil {
		t.Fatal(err)
	}

	second, err := slab.Alloc()
	if err != nil {
		t.Fatal(err)
	}

	if second != first || second.payload != 42 {
		t.Fatalf("reused object = %p/%d, want %p/42", second, second.payload, first)
	}
}

func TestSlabReturnsOutOfMemoryWhenFull(t *testing.T) {
	slab := newTestSlab(t, 1)
	if _, err := slab.Alloc(); err != nil {
		t.Fatal(err)
	}

	if _, err := slab.Alloc(); !errors.Is(err, ErrOutOfMemory) {
		t.Fatalf("second Alloc() = %v, want %v", err, ErrOutOfMemory)
	}
}

func TestSlabPropagatesDoubleFree(t *testing.T) {
	slab := newTestSlab(t, 1)
	obj, err := slab.Alloc()
	if err != nil {
		t.Fatal(err)
	}

	if err := slab.Free(obj); err != nil {
		t.Fatal(err)
	}

	if err := slab.Free(obj); !errors.Is(err, ErrAlreadyFree) {
		t.Fatalf("second Free() = %v, want %v", err, ErrAlreadyFree)
	}
}

func TestSlabRejectsAllocationAfterRelease(t *testing.T) {
	slab := newTestSlab(t, 1)
	if err := slab.Release(); err != nil {
		t.Fatal(err)
	}

	if _, err := slab.Alloc(); !errors.Is(err, ErrReleased) {
		t.Fatalf("Alloc() after release = %v, want %v", err, ErrReleased)
	}
}

func TestSlabRejectsFreeAfterRelease(t *testing.T) {
	slab := newTestSlab(t, 1)
	if err := slab.Release(); err != nil {
		t.Fatal(err)
	}

	if err := slab.Free(nil); !errors.Is(err, ErrReleased) {
		t.Fatalf("Free() after release = %v, want %v", err, ErrReleased)
	}
}

func TestSlabRejectsSecondRelease(t *testing.T) {
	slab := newTestSlab(t, 1)
	if err := slab.Release(); err != nil {
		t.Fatal(err)
	}

	if err := slab.Release(); !errors.Is(err, ErrReleased) {
		t.Fatalf("second Release() = %v, want %v", err, ErrReleased)
	}
}
