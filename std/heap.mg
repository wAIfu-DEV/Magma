mod heap

use "allocator.mg" a
use "errors.mg"    e
use "cast.mg"      cast

ext ext_malloc  malloc(size u64) ptr
ext ext_realloc realloc(block ptr, newSize u64) ptr
ext ext_free    free(block ptr) void

HeapAllocator(
    _ ptr,
)

heapAlloc(impl ptr, nBytes u64) !u8*:
    p ptr = ext_malloc(nBytes)

    if cast.ptou(p) == 0:
        if nBytes == 0:
            throw e.errInvalidArgument("requested size is 0")
        ..
        throw e.errOutOfMemory("OOM")
    ..
    ret p
..

heapRealloc(impl ptr, in u8*, nBytes u64) !u8*:
    p ptr = ext_realloc(in, nBytes)

    if cast.ptou(p) == 0:
        if cast.ptou(in) == 0:
            throw e.errInvalidArgument("input pointer is null")
        ..
        if nBytes == 0:
            throw e.errInvalidArgument("requested size is 0")
        ..
        throw e.errOutOfMemory("OOM")
    ..
    ret p
..

heapFree(impl ptr, in u8*) void:
    if cast.ptou(in) == 0:
        ret
    ..
    ext_free(in)
..

HeapAllocator.allocator() a.Allocator:
    alloc a.Allocator

    alloc.impl = this
    alloc.fn_alloc = heapAlloc
    alloc.fn_realloc = heapRealloc
    alloc.fn_free = heapFree

    ret alloc
..

pub alloc(nBytes u64) !u8*:
    ret try heapAlloc(cast.utop(0), nBytes)
..

pub realloc(in u8*, nBytes u64) !u8*:
    ret try heapRealloc(cast.utop(0), in, nBytes)
..

pub free(in u8*) void:
    heapFree(cast.utop(0), in)
..
