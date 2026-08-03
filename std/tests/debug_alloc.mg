mod main

use "std:debug_alloc" debug_alloc
use "std:errors" errors
use "std:heap" heap

pub main() !void:
    options := debug_alloc.Options(initialCapacity=1, canGrow=true, rejectUntrackedFree=true)
    debug := try debug_alloc.new(heap.allocator(), options)
    defer debug.free()
    a := debug.allocator()

    first := try a.alloc(16)
    second := try a.alloc(32)
    first[0] = 91
    first = try a.realloc(first, 48)
    if first[0] != 91:
        throw errors.failure("debug realloc did not preserve data")
    ..

    stats := debug.stats()
    if stats.liveAllocations != 2 || stats.liveBytes != 80 || stats.reallocationCalls != 1:
        throw errors.failure("debug allocation statistics are incorrect")
    ..
    leak := try debug.leak(0)
    if leak.pointer == none || leak.size == 0:
        throw errors.failure("debug leak inspection failed")
    ..

    a.free(first)
    a.free(first)
    if debug.stats().rejectedFrees != 1:
        throw errors.failure("debug allocator did not reject a double free")
    ..
    a.free(second)
    if debug.stats().liveAllocations != 0 || debug.stats().liveBytes != 0:
        throw errors.failure("debug allocator retained freed allocations")
    ..
..
