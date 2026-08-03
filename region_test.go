package pact

import (
	"errors"
	"testing"
)

func newTestRegion(t *testing.T, size uintptr) *region {
	t.Helper()
	r, err := newRegion(size)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if r.addr == nil {
			return
		}

		if err := r.release(); err != nil {
			t.Errorf("cleanup release: %v", err)
		}
	})
	return r
}

func TestNewRegionRejectsZeroSize(t *testing.T) {
	if _, err := newRegion(0); !errors.Is(err, ErrInvalidReserveSize) {
		t.Fatalf("newRegion(0) = %v, want %v", err, ErrInvalidReserveSize)
	}
}

func TestNewRegionRejectsReservationSizeOverflow(t *testing.T) {
	if _, err := newRegion(maxUintptr - 1); !errors.Is(err, ErrOutOfMemory) {
		t.Fatalf("newRegion() = %v, want %v", err, ErrOutOfMemory)
	}
}

func TestRegionStartsUncommitted(t *testing.T) {
	r := newTestRegion(t, pageSize)
	if r.committed != 0 {
		t.Fatalf("new region committed = %d, want 0", r.committed)
	}
}

func TestRegionAddressAppliesAlignment(t *testing.T) {
	r := newTestRegion(t, pageSize)
	ptr, next, err := r.address(1, 1, 8)
	if err != nil {
		t.Fatal(err)
	}

	if uintptr(ptr)%8 != 0 || next != 9 {
		t.Fatalf("address = %#x, next = %d; want aligned address and next 9", uintptr(ptr), next)
	}
}

func TestRegionAddressCommitsPagesLazily(t *testing.T) {
	r := newTestRegion(t, pageSize*2)
	if _, _, err := r.address(1, 1, 1); err != nil {
		t.Fatal(err)
	}

	if r.committed != pageSize {
		t.Fatalf("committed after first address = %d, want %d", r.committed, pageSize)
	}

	if _, _, err := r.address(pageSize, 1, 1); err != nil {
		t.Fatal(err)
	}

	if r.committed != pageSize*2 {
		t.Fatalf("committed after page boundary = %d, want %d", r.committed, pageSize*2)
	}
}

func TestRegionAddressRejectsZeroSize(t *testing.T) {
	r := newTestRegion(t, pageSize)
	if _, _, err := r.address(0, 0, 1); !errors.Is(err, ErrInvalidSize) {
		t.Fatalf("address() = %v, want %v", err, ErrInvalidSize)
	}
}

func TestRegionAddressRejectsPastReservation(t *testing.T) {
	r := newTestRegion(t, pageSize)
	if _, _, err := r.address(pageSize, 1, 1); !errors.Is(err, ErrOutOfMemory) {
		t.Fatalf("address() = %v, want %v", err, ErrOutOfMemory)
	}
}

func TestRegionAddressRejectsArithmeticOverflow(t *testing.T) {
	r := newTestRegion(t, pageSize)
	if _, _, err := r.address(maxUintptr, 1, 1); !errors.Is(err, ErrOutOfMemory) {
		t.Fatalf("address() = %v, want %v", err, ErrOutOfMemory)
	}

	if _, _, err := r.address(maxUintptr-3, 1, 8); !errors.Is(err, ErrOutOfMemory) {
		t.Fatalf("aligned address() = %v, want %v", err, ErrOutOfMemory)
	}
}

func TestRegionTrimKeepsPagesBeforeOffset(t *testing.T) {
	r := newTestRegion(t, pageSize*2)
	if _, _, err := r.address(pageSize, pageSize, 1); err != nil {
		t.Fatal(err)
	}

	if err := r.trim(1); err != nil {
		t.Fatal(err)
	}

	if r.committed != pageSize {
		t.Fatalf("committed after trim = %d, want %d", r.committed, pageSize)
	}
}

func TestRegionReleaseClearsState(t *testing.T) {
	r := newTestRegion(t, pageSize)
	if err := r.release(); err != nil {
		t.Fatal(err)
	}

	if r.addr != nil || r.reserved != 0 || r.committed != 0 {
		t.Fatalf("released region state = %#v, want zero state", r)
	}
}
