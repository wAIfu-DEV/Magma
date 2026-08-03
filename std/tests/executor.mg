mod main

use "std:allocator" allocator
use "std:errors" errors
use "std:executor" executor
use "std:heap" heap
use "std:thread_pool" thread_pool

increment(value u64*) u64:
    *value = *value + 1
    ret 0
..

submitIncrement(scheduler executor.Executor*, value u64*) !void:
    try scheduler.submit[u64](increment, value)
..

pub main() !void:
    a allocator.Allocator = heap.allocator()
    pool := try thread_pool.new(a, 1, 1, 4, 1)
    scheduler := pool.executor()
    value u64 = 0

    try submitIncrement(addrof scheduler, addrof value)
    try pool.wait()
    if value != 1:
        try pool.close()
        throw errors.failure("executor did not submit work to its thread pool")
    ..

    try pool.close()
..
