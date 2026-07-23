mod heap
# Deterministic allocator used to exercise allocation-failure handling.
# @warning Intended for testing rather than production allocation.

use "std:allocator" a
use "std:errors"    e

# Internals for alloc, used by both alloc() and HeapAllocator.alloc()
# @complexity O(1) for allocation itself, O(N) for zeroing if requested.
fakeAlloc(impl ptr, nBytes u64) !$u8*:
    throw e.failure("fake alloc")
    ret none
..

# Internals for realloc, used by both realloc() and HeapAllocator.realloc()
# @complexity O(1) for reallocation itself.
fakeRealloc(impl ptr, in u8*, nBytes u64) !$u8*:
    throw e.failure("fake realloc")
    ret none
..

# Internals for free, used by both free() and HeapAllocator.free()
# @complexity O(1).
fakeFree(impl ptr, in u8*) void:
    ret
..

const gl_fakeVtable := a.Vtable(
    fn_alloc =   fakeAlloc,
    fn_realloc = fakeRealloc,
    fn_free =    fakeFree,
)

# Returns an allocator whose allocation and reallocation operations always fail.
# Free is accepted as a no-op, allowing failure-path tests to use normal cleanup.
# @complexity O(1).
# @returns non-owning deterministic failure allocator
# @example
#   a := fake_alloc.allocator()
#   block u8*, allocationError error = a.alloc(16)
pub allocator() a.Allocator:
    ret a.Allocator(impl=none, vtable=addrof gl_fakeVtable)
..
