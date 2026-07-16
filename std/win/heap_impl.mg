mod heap_impl_win

use "../allocator.mg" a
use "../errors.mg"    e
use "../cast.mg"      cast
use "../memory.mg"    mem

# Windows heap API
ext ext_win32_GetProcessHeap GetProcessHeap() ptr
ext ext_win32_HeapAlloc      HeapAlloc(hHeap ptr, dwFlags u32, dwBytes u64) ptr
ext ext_win32_HeapReAlloc    HeapReAlloc(hHeap ptr, dwFlags u32, lpMem ptr, dwBytes u64) ptr
ext ext_win32_HeapFree       HeapFree(hHeap ptr, dwFlags u32, lpMem ptr) u32

# Heap handle caching for performance
gl_heap ptr

# Gets the process heap handle, cached for performance
# O(1) after first call.
getHeap() ptr:
    if gl_heap == none:
        gl_heap = ext_win32_GetProcessHeap()
    ..
    ret gl_heap
..

# Internals for alloc, used by both alloc() and HeapAllocator.alloc()
# O(1) for allocation itself, O(N) for zeroing if requested.
heapAlloc(impl ptr, nBytes u64) !$u8*:
    if nBytes == 0:
        throw e.invalidArgument("requested size is 0")
    ..

    heap ptr = getHeap()
    p ptr = ext_win32_HeapAlloc(heap, 0, nBytes)

    if p == none:
        throw e.outOfMemory("OOM")
    ..
    ret p
..

# Internals for realloc, used by both realloc() and HeapAllocator.realloc()
# O(1) for reallocation itself.
heapRealloc(impl ptr, in u8*, nBytes u64) !$u8*:
    if in == none:
        throw e.invalidArgument("input pointer is null")
    ..

    if nBytes == 0:
        throw e.invalidArgument("requested size is 0")
    ..

    heap ptr = getHeap()
    p ptr = ext_win32_HeapReAlloc(heap, 0, in, nBytes)

    if p == none:
        throw e.outOfMemory("OOM")
    ..
    ret p
..

# Internals for free, used by both free() and HeapAllocator.free()
# O(1).
heapFree(impl ptr, in u8*) void:
    if in == none:
        ret
    ..

    heap ptr = getHeap()
    ok u32 = ext_win32_HeapFree(heap, 0, in)
    if ok == 0:
        ret
    ..
..

const gl_heapVtable := a.Vtable(
    fn_alloc =   heapAlloc,
    fn_realloc = heapRealloc,
    fn_free =    heapFree,
)

# Returns an allocator object that uses Windows heap allocation.
# O(1).
pub allocator() a.Allocator:
    ret a.Allocator(impl=none, vtable=addrof gl_heapVtable)
..

# Returns a heap-allocated region of memory of exactly nBytes bytes wide.
# nBytes should be greater than 0.
# Warning: the region of memory is owned by the caller, meaning the caller is
# responsible for either deallocating, or explicitly transfering ownership to
# consuming clients.
# Warning: returned memory region will be uninitialized, do not rely on assumptions
# as to what may be present inside, it is the caller's responsibility to initialize
# the memory region. If you want the memory region to be zeroed-out, use allocZero()
# @param nBytes how many bytes to allocate
# @returns owned region of memory
pub alloc(nBytes u64) !$u8*:
    ret try heapAlloc(none, nBytes)
..

# Returns a heap-allocated region of memory of exactly nBytes bytes wide.
# Returned memory region will be zeroed-out.
# nBytes should be greater than 0.
# Warning: the region of memory is owned by the caller, meaning the caller is
# responsible for either deallocating, or explicitly transfering ownership to
# consuming clients.
# @param nBytes how many bytes to allocate
# @returns owned region of memory
pub allocZero(nBytes u64) !$u8*:
    out ptr = try heapAlloc(none, nBytes)
    mem.zero(out, nBytes)
    ret out
..

# Returns a heap-allocated region of memory of exactly nBytes bytes wide.
# Bytes pointed to by the "in" pointer will be copied to the new region.
# in should be non-null, and should be the result of an allocation from this
# module's allocator or methods, do not mismatch allocators.
# nBytes should be greater than 0.
# Warning: the region of memory is owned by the caller, meaning the caller is
# responsible for either deallocating or explicitly transfering ownership to
# consuming clients.
# Warning: returned memory region may have its end uninitialized, do not rely on
# assumptions as to what may be present inside, it is the caller's responsibility
# to initialize the memory region past previous size if nBytes > previous size.
# If you want the memory region to be zeroed-out, use allocZero()
# @param in pointer to already allocated memory region
# @param nBytes how many bytes to allocate
# @returns owned region of memory
pub realloc(in u8*, nBytes u64) !$u8*:
    ret try heapRealloc(none, in, nBytes)
..

# Returns a heap-allocated region of memory of exactly nBytes bytes wide.
# Bytes pointed to by the "in" pointer will be copied to the new region.
# in should be non-null, and should be the result of an allocation from this
# module's allocator or methods, do not mismatch allocators.
# nBytes should be greater than 0.
# Warning: the region of memory is owned by the caller, meaning the caller is
# responsible for either deallocating or explicitly transfering ownership to
# consuming clients.
# @param in pointer to already allocated memory region
# @param nBytes how many bytes to allocate
# @param prevNbytes previous size of the allocation
# @returns owned region of memory
pub reallocZero(in u8*, nBytes u64, prevNbytes u64) !$u8*:
    if nBytes <= prevNbytes:
        ret try heapRealloc(none, in, nBytes)
    ..
    # manual realloc
    out ptr = try heapAlloc(none, nBytes)
    mem.copy(in, out, prevNbytes)

    # zero end of region
    outEnd ptr = cast.utop(cast.ptou(out) + prevNbytes)
    mem.zero(outEnd, nBytes - prevNbytes)

    heapFree(none, in)
    ret out
..

# Deallocates a heap-allocated region of memory.
# in should be non-null, and should be the result of an allocation from this
# module's allocator or methods, do not mismatch allocators.
# @param in pointer to already allocated memory region
pub free(in u8*) void:
    heapFree(none, in)
..
