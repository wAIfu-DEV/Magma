mod heap

use "allocator.mg" allocator
use "errors.mg"    errors

llvm "declare ptr @malloc(i64)\n"
llvm "declare ptr @realloc(ptr, i64)\n"
llvm "declare void @free(ptr)\n"

HeapAllocator()

heapAlloc(impl ptr, nBytes u64) !u8*:
    p ptr

    llvm "  %p1 = call ptr @malloc(i64 %nBytes)\n"
    llvm "  store ptr %p1, ptr %p\n"

    if p == 0:
        if nBytes == 0:
            throw errors.invalidArgument("requested size is 0")
        ..
        throw errors.outOfMemory("")
    ..
    ret p
..

heapAlloc(impl ptr, in u8*, nBytes u64) !u8*:
    p ptr

    llvm "  %p1 = call ptr @realloc(ptr %in, i64 %nBytes)\n"
    llvm "  store ptr %p1, ptr %p\n"

    if p == 0:
        if in == 0:
            throw errors.invalidArgument("input pointer is null")
        ..
        if nBytes == 0:
            throw errors.invalidArgument("requested size is 0")
        ..
        throw errors.outOfMemory("")
    ..
    ret p
..

heapFree(impl ptr, in u8*) !void:
    if in == 0:
        throw errors.invalidArgument("input pointer is null")
    ..

    llvm "  call void @free(ptr %in)\n"
..

HeapAllocator.allocator() allocator.Allocator:
    a allocator.Allocator

    a.impl = this
    a.fn_alloc = heapAlloc
    a.fn_realloc = heapRealloc
    a.fn_free = heapFree

    ret a
..

pub alloc(nBytes u64) !u8*:
    p ptr
    ret try heapAlloc(p, nBytes)
..

pub realloc(in u8*, nBytes u64) !u8*:
    p ptr
    ret try heapRealloc(p, in, nBytes)
..

pub free(in u8*) !void:
    p ptr
    ret try heapFree(p, in)
..
