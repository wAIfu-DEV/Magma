mod heap

use "allocator.mg" a
use "errors.mg"    e
use "cast.mg"      cast

llvm "declare ptr @malloc(i64)\n"
llvm "declare ptr @realloc(ptr, i64)\n"
llvm "declare void @free(ptr)\n"

HeapAllocator(
    _ ptr,
)

heapAlloc(impl ptr, nBytes u64) !u8*:
    p ptr

    llvm "  %p1 = call ptr @malloc(i64 %nBytes)\n"
    llvm "  store ptr %p1, ptr %p\n"

    if cast.ptou(p) == 0:
        if nBytes == 0:
            throw e.invalidArgument("requested size is 0")
        ..
        throw e.outOfMemory("OOM")
    ..
    ret p
..

heapRealloc(impl ptr, in u8*, nBytes u64) !u8*:
    p ptr

    llvm "  %p1 = call ptr @realloc(ptr %in, i64 %nBytes)\n"
    llvm "  store ptr %p1, ptr %p\n"

    if cast.ptou(p) == 0:
        if cast.ptou(in) == 0:
            throw e.invalidArgument("input pointer is null")
        ..
        if nBytes == 0:
            throw e.invalidArgument("requested size is 0")
        ..
        throw e.outOfMemory("OOM")
    ..
    ret p
..

heapFree(impl ptr, in u8*) void:
    if cast.ptou(in) == 0:
        ret
    ..
    llvm "  call void @free(ptr %in)\n"
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
