package pact

import (
	"errors"
	"testing"
)

func TestMust(t *testing.T) {
	if got := Must(42, nil); got != 42 {
		t.Fatalf("Must() = %d, want 42", got)
	}

	want := ErrOutOfMemory
	defer func() {
		if got := recover(); got != want {
			t.Fatalf("Must() panic = %v, want %v", got, want)
		}
	}()
	Must(0, want)
}

func TestAlignUp(t *testing.T) {
	for _, test := range []struct {
		x, align, want uintptr
	}{
		{x: 0, align: 1, want: 0},
		{x: 7, align: 8, want: 8},
		{x: 8, align: 8, want: 8},
		{x: 9, align: 8, want: 16},
		{x: 17, align: 16, want: 32},
	} {
		if got := alignUp(test.x, test.align); got != test.want {
			t.Errorf("alignUp(%d, %d) = %d, want %d", test.x, test.align, got, test.want)
		}
	}
}

func TestAlignOffsetUsesAbsoluteAddress(t *testing.T) {
	base := pageSize + 1
	align := pageSize * 2
	offset, err := alignOffset(base, 0, align)
	if err != nil {
		t.Fatal(err)
	}

	if (base+offset)%align != 0 {
		t.Fatalf("aligned address = %d, want a multiple of %d", base+offset, align)
	}
}

func TestAllocSliceAllowsEmptySlice(t *testing.T) {
	slice, err := AllocSlice[uint64](nil, 0)
	if err != nil {
		t.Fatal(err)
	}

	if len(slice) != 0 || cap(slice) != 0 {
		t.Fatalf("empty slice = len %d, cap %d", len(slice), cap(slice))
	}
}

func TestAllocNewAllowsZeroSizedType(t *testing.T) {
	value, err := AllocNew[struct{}](nil)
	if err != nil {
		t.Fatal(err)
	}

	if value != nil {
		t.Fatalf("zero-sized value = %p, want nil", value)
	}
}

func TestAllocStringAllowsEmptyString(t *testing.T) {
	value, err := AllocString(nil, "")
	if err != nil {
		t.Fatal(err)
	}

	if value != "" {
		t.Fatalf("empty string = %q", value)
	}
}

func TestAllocSliceRejectsSizeOverflow(t *testing.T) {
	if _, err := AllocSlice[uint64](nil, 0, maxUintptr); !errors.Is(err, ErrOutOfMemory) {
		t.Fatalf("AllocSlice() = %v, want %v", err, ErrOutOfMemory)
	}
}

func TestAllocSliceRejectsLengthConversionOverflow(t *testing.T) {
	if _, err := AllocSlice[struct{}](nil, 0, maxIntValue+1); !errors.Is(err, ErrOutOfMemory) {
		t.Fatalf("AllocSlice() = %v, want %v", err, ErrOutOfMemory)
	}
}
