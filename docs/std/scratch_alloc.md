# `std/scratch_alloc`

Provides a fixed-capacity free-list allocator. It splits free blocks, reuses
individually freed storage, and coalesces adjacent free blocks. The allocator
never grows beyond its backing region.

Use `new(allocator, capacity)` for a chosen capacity, `newDefault(allocator)`
for 64 KiB, or `fromBuffer(buffer)` for caller-owned storage. `reset()` releases
all allocations at once. The `destroy()` destructor releases owned backing
storage but not a buffer supplied through `fromBuffer`. Individual calls to the
`Allocator.free` view return blocks to the scratch allocator's free list.

The name "scratch allocator" replaces the proposed "buffer allocator" to
describe its intended temporary-workspace role without confusing it with byte
buffer containers.
