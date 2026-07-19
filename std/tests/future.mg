mod main

use "../allocator.mg" allocator
use "../errors.mg" errors
use "../future.mg" future
use "../heap.mg" heap
use "../reader.mg" reader
use "../strings.mg" strings
use "../thread.mg" thread
use "../thread_pool.mg" thread_pool

source(impl ptr, bytes u8[], count u64) !u64:
    if count > 0:
        bytes[0] = 65
        ret 1
    ..
    ret 0
..

failingSource(impl ptr, bytes u8[], count u64) !u64:
    throw errors.failure("asynchronous read failed")
..

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

    input := reader.new(none, source)
    pending := try input.readAsync(pool, a, 1)

    done bool = try pending.isDone()
    while done == false:
        thread.yield()
        done = try pending.isDone()
    ..

    result := try pending.await()
    if strings.compare(result, "A") == false:
        strings.free(a, result)
        try pool.close()
        throw errors.failure("Future returned the wrong reader result")
    ..
    strings.free(a, result)

    failing := reader.new(none, failingSource)
    failed := try failing.readAsync(pool, a, 1)
    failedValue str, failedError error = failed.await()
    if failedError.ok():
        strings.free(a, failedValue)
        try pool.close()
        throw errors.failure("Future did not propagate the worker error")
    ..

    try pool.close()
..
