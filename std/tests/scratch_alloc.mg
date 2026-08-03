mod main

use "std:scratch_alloc" scratch_alloc
use "std:errors" errors
use "std:heap" heap

pub main() !void:
    scratch := try scratch_alloc.new(heap.allocator(), 1024)
    defer scratch.free()
    a := scratch.allocator()

    first := try a.alloc(64)
    second := try a.alloc(64)
    first[0] = 73
    a.free(second)
    first = try a.realloc(first, 160)
    if first[0] != 73:
        throw errors.failure("scratch realloc did not preserve data")
    ..

    a.free(first)
    large := try a.alloc(800)
    if large == none:
        throw errors.failure("scratch coalescing failed")
    ..
    a.free(large)
    scratch.reset()
..
