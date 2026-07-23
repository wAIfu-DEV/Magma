mod main

use "std:allocator" allocator
use "std:errors" errors
use "std:future" future
use "std:heap" heap
use "std:thread_pool" thread_pool

doubleValue(context u64*) !u64:
    ret *context * 2
..

pub main() !void:
    a allocator.Allocator = heap.allocator()
    pool := try thread_pool.new(a, 1, 1, 8, 1)

    direct := try future.new[u64, u64](a, pool, doubleValue, 21)
    if try direct.await() != 42:
        try pool.close()
        throw errors.failure("direct Future returned the wrong value")
    ..

    try pool.close()
..
