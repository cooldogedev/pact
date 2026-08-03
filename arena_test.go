package pact

import (
	"errors"
	"testing"
)

func newTestArena(t *testing.T, size uintptr) *Arena {
	t.Helper()
	arena, err := NewArena(size)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if arena.reg == nil {
			return
		}

		if err := arena.Release(); err != nil {
			t.Errorf("cleanup release: %v", err)
		}
	})
	return arena
}

func TestNewArenaRejectsZeroSize(t *testing.T) {
	if _, err := NewArena(0); !errors.Is(err, ErrInvalidReserveSize) {
		t.Fatalf("NewArena(0) error = %v, want %v", err, ErrInvalidReserveSize)
	}
}

func TestArenaAllocatesNonOverlappingAlignedMemory(t *testing.T) {
	arena := newTestArena(t, pageSize)
	first, err := arena.Alloc(1, 1)
	if err != nil {
		t.Fatal(err)
	}

	second, err := arena.Alloc(1, 8)
	if err != nil {
		t.Fatal(err)
	}

	if uintptr(second)%8 != 0 {
		t.Fatalf("second allocation %p is not 8-byte aligned", second)
	}

	*(*byte)(first) = 0xaa
	*(*byte)(second) = 0x55
	if *(*byte)(first) != 0xaa || *(*byte)(second) != 0x55 {
		t.Fatal("allocations overlap")
	}
}

func TestArenaAllocatesValuesAndSlices(t *testing.T) {
	type arenaValue struct {
		count uint64
		flag  byte
	}

	arena := newTestArena(t, pageSize)
	value, err := AllocNew[arenaValue](arena)
	if err != nil {
		t.Fatal(err)
	}

	value.count, value.flag = 42, 1
	values, err := AllocSlice[arenaValue](arena, 2, 4)
	if err != nil {
		t.Fatal(err)
	}

	values[0].count = 7
	values[1].count = 11
	if value.count != 42 || value.flag != 1 || len(values) != 2 || cap(values) != 4 || values[1].count != 11 {
		t.Fatalf(
			"allocated value/slice not usable: value=%+v len=%d cap=%d values=%+v",
			*value, len(values), cap(values), values,
		)
	}
}

func TestArenaAllocatesString(t *testing.T) {
	arena := newTestArena(t, pageSize)
	got, err := AllocString(arena, "pact")
	if err != nil {
		t.Fatal(err)
	}

	if got != "pact" {
		t.Fatalf("allocated string = %q, want %q", got, "pact")
	}
}

func TestArenaObjectsCanReferenceEachOther(t *testing.T) {
	type arenaNode struct {
		value uint64
		next  *arenaNode
	}

	arena := newTestArena(t, pageSize)
	first, err := AllocNew[arenaNode](arena)
	if err != nil {
		t.Fatal(err)
	}

	second, err := AllocNew[arenaNode](arena)
	if err != nil {
		t.Fatal(err)
	}

	first.value, second.value = 1, 2
	first.next = second
	if first.next != second {
		t.Fatalf("arena object link = %p, want %p", first.next, second)
	}

	if first.next.value != 2 {
		t.Fatalf("linked arena object value = %d, want 2", first.next.value)
	}
}

func TestArenaCommitsPagesOnDemand(t *testing.T) {
	arena := newTestArena(t, pageSize*2)
	if arena.reg.committed != 0 {
		t.Fatalf("new arena committed = %d, want 0", arena.reg.committed)
	}

	if _, err := arena.Alloc(1, 1); err != nil {
		t.Fatal(err)
	}

	if arena.reg.committed != pageSize {
		t.Fatalf("committed after first allocation = %d, want %d", arena.reg.committed, pageSize)
	}

	if _, err := arena.Alloc(pageSize, 1); err != nil {
		t.Fatal(err)
	}

	if arena.reg.committed != pageSize*2 {
		t.Fatalf("committed after boundary crossing = %d, want %d", arena.reg.committed, pageSize*2)
	}
}

func TestArenaResetReusesMemory(t *testing.T) {
	arena := newTestArena(t, pageSize)
	first, err := arena.Alloc(1, 1)
	if err != nil {
		t.Fatal(err)
	}

	*(*byte)(first) = 99
	if err := arena.Reset(); err != nil {
		t.Fatal(err)
	}

	second, err := arena.Alloc(1, 1)
	if err != nil {
		t.Fatal(err)
	}

	if second != first || *(*byte)(second) != 99 {
		t.Fatalf("reset did not reuse preserved memory: got %p/%d, want %p/99", second, *(*byte)(second), first)
	}
}

func TestArenaTrimDecommitsUnusedPages(t *testing.T) {
	arena := newTestArena(t, pageSize*2)
	if _, err := arena.Alloc(pageSize+1, 1); err != nil {
		t.Fatal(err)
	}

	if err := arena.Reset(); err != nil {
		t.Fatal(err)
	}

	if err := arena.Trim(); err != nil {
		t.Fatal(err)
	}

	if arena.reg.committed != 0 {
		t.Fatalf("committed after trim = %d, want 0", arena.reg.committed)
	}
}

func TestArenaRejectsZeroSizeAllocation(t *testing.T) {
	arena := newTestArena(t, pageSize)
	if _, err := arena.Alloc(0, 1); !errors.Is(err, ErrInvalidSize) {
		t.Fatalf("Alloc(0) error = %v, want %v", err, ErrInvalidSize)
	}
}

func TestArenaRejectsInvalidAlignment(t *testing.T) {
	arena := newTestArena(t, pageSize)
	if _, err := arena.Alloc(1, 3); !errors.Is(err, ErrInvalidAlignment) {
		t.Fatalf("Alloc() = %v, want %v", err, ErrInvalidAlignment)
	}
}

func TestArenaRejectsAllocationAfterRelease(t *testing.T) {
	arena := newTestArena(t, pageSize)
	if err := arena.Release(); err != nil {
		t.Fatal(err)
	}

	if _, err := arena.Alloc(1, 1); !errors.Is(err, ErrReleased) {
		t.Fatalf("Alloc() after release = %v, want %v", err, ErrReleased)
	}
}

func TestArenaRejectsSecondRelease(t *testing.T) {
	arena := newTestArena(t, pageSize)
	if err := arena.Release(); err != nil {
		t.Fatal(err)
	}

	if err := arena.Release(); !errors.Is(err, ErrReleased) {
		t.Fatalf("second Release() = %v, want %v", err, ErrReleased)
	}
}
