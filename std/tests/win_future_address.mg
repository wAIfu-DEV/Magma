mod main

use "../allocator.mg" allocator
use "../errors.mg" errors
use "../future.mg" future
use "../heap.mg" heap
use "../thread.mg" thread
use "../thread_pool.mg" thread_pool

Context(
    value u64
    fail bool
)

produce(context Context*) !u64:
    if context.fail:
        throw errors.failure("address Future failure")
    ..
    ret context.value
..

pub main() !void:
    a allocator.Allocator = heap.allocator()
    pool := try thread_pool.new(a, 1, 8)

    pending := try future.new[u64, Context](a, pool, produce, Context(value=42, fail=false))
    done bool = try pending.isDone()
    while done == false:
        thread.yield()
        done = try pending.isDone()
    ..
    value u64 = try pending.await()
    if value != 42:
        try pool.close()
        throw errors.failure("address Future returned the wrong value")
    ..

    failed := try future.new[u64, Context](a, pool, produce, Context(value=0, fail=true))
    failedValue u64, failure error = failed.await()
    if errors.code(failure) == 0:
        try pool.close()
        throw errors.failure("address Future did not propagate its error")
    ..
    try pool.close()
..
