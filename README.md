# pact

pact is a page allocation & commitment toolkit for Go.

It reserves virtual memory up front and commits pages as they are used. The
package provides three allocators:

- `Arena` for fast, ordered allocations with reset support.
- `Pool` for fixed-size blocks that can be returned and reused.
- `Slab[T]` for typed objects backed by a pool.

## Usage

Use an arena when a group of allocations shares one lifetime:

```go
type entry struct {
	ID    uint64
	Value uint32
}

arena, err := pact.NewArena(64 << 20)
if err != nil {
	return err
}
defer arena.Release()

entries, err := pact.AllocSlice[entry](arena, 0, 1024)
if err != nil {
	return err
}

entries = entries[:1]
entries[0].ID = 1
entries[0].Value = 42
```

For setup code where allocation failure is unrecoverable, `Must` keeps the
same usage concise. `Slab` is the usual choice for typed objects; `Pool` is a
lower-level raw block allocator:

```go
type item struct {
	ID    uint64
	Value uint32
}

slab := pact.Must(pact.NewSlab[item](1024))
defer slab.Release()

value := pact.Must(slab.Alloc())
value.ID = 1
```

## Memory model

Memory returned by pact is outside the Go heap. It is not guaranteed to be
cleared on allocation, and pointers stored in it do not keep ordinary Go heap
objects alive.

Objects allocated from the same arena, pool, or slab may reference one
another while both objects remain allocated. This is an address-lifetime
guarantee, not Go-managed object ownership.

The allocators do not synchronise access. They may be shared between
goroutines, provided that all access is synchronised by the caller.
