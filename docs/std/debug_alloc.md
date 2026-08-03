# `std/debug_alloc`

Wraps another allocator and records live allocations, peak usage, operation
counts, metadata cost, and rejected operations. `leakCount()` and `leak(index)`
provide allocation-free leak inspection.

Use `new(target, options)` to select initial capacity, table growth, and
untracked-free behavior. `newDefault(target)` starts with 256 entries, grows as
needed, and rejects untracked frees.

Unlike the XSTD and Wade32 versions, an unknown pointer is checked before the
target allocator is called. Tracking capacity is reserved before allocation,
and realloc updates its existing entry, so a successful target realloc cannot
be invalidated by later bookkeeping failure.

The wrapper is not synchronized. Use external locking when sharing one debug
allocator between threads. `free()` releases tracking metadata but deliberately
does not release reported live allocations.
