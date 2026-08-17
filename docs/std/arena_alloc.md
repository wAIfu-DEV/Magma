# `std/arena_alloc`

Provides a bump allocator for groups of allocations that share a lifetime.
Individual `free` calls are no-ops; `Arena.reset()` releases all allocations.

Use `new(allocator, capacity)` for a chosen capacity, `newDefault(allocator)`
for 64 KiB, or `fromBuffer(buffer)` for caller-owned storage. Call
`Arena.allocator()` only after the arena is stored at its final address because
the returned interface borrows it.

`Arena.destroy()` is the destructor. It releases owned backing storage and does
not free storage supplied through `fromBuffer`.

`Arena.used()` returns the number of bytes consumed since construction or the
last reset. `Arena.capacity()` returns total backing-storage capacity.
