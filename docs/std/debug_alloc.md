# `std/debug_alloc`

Wraps another allocator and records live allocations, peak usage, operation
counts, metadata cost, and rejected operations. `leakCount()` and `leak(index)`
provide allocation-free leak inspection.

Use `new(target, options)` to select initial capacity, table growth, and
untracked-free behavior. `newDefault(target)` starts with 256 entries, grows as
needed, and rejects untracked frees.

`DebugAllocator.allocator()` returns the instrumented allocator interface.
`DebugAllocator.stats()` returns a `Stats` snapshot containing live and peak
allocation counts and bytes, allocation/reallocation/free call counts,
metadata bytes, and rejected-operation counts. `Leak` describes one tracked
live allocation. The returned interface borrows the `DebugAllocator`, which
must remain at a stable address and outlive its users.

Unlike the XSTD and Wade32 versions, an unknown pointer is checked before the
target allocator is called. Tracking capacity is reserved before allocation,
and realloc updates its existing entry, so a successful target realloc cannot
be invalidated by later bookkeeping failure.

The wrapper is not synchronized. Use external locking when sharing one debug
allocator between threads. `free()` releases tracking metadata but deliberately
does not release reported live allocations.
