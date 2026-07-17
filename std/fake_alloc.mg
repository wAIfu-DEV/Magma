mod heap

use "allocator.mg" a
use "errors.mg"    e

# Internals for alloc, used by both alloc() and HeapAllocator.alloc()
# O(1) for allocation itself, O(N) for zeroing if requested.
fakeAlloc(impl ptr, nBytes u64) !$u8*:
    throw e.failure("fake alloc")
    ret none
..

# Internals for realloc, used by both realloc() and HeapAllocator.realloc()
# O(1) for reallocation itself.
fakeRealloc(impl ptr, in u8*, nBytes u64) !$u8*:
    throw e.failure("fake realloc")
    ret none
..

# Internals for free, used by both free() and HeapAllocator.free()
# O(1).
fakeFree(impl ptr, in u8*) void:
    ret
..

const gl_fakeVtable := a.Vtable(
    fn_alloc =   fakeAlloc,
    fn_realloc = fakeRealloc,
    fn_free =    fakeFree,
)

# Returns an allocator object that uses Windows heap allocation.
# O(1).
pub allocator() a.Allocator:
    ret a.Allocator(impl=none, vtable=addrof gl_fakeVtable)
..
