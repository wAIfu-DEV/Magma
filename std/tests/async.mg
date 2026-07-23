mod main

use "std:allocator" allocator
use "std:async" async
use "std:errors" errors
use "std:heap" heap
use "std:reader" reader
use "std:strings" strings
use "std:thread" thread
use "std:thread_pool" thread_pool

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

pub main() !void:
    a allocator.Allocator = heap.allocator()
    pool := try thread_pool.new(a, 1, 1, 8, 1)
    as := async.new(pool, a)

    input := reader.new(none, source)
    pending := try as.read(input, 1)

    done bool = try pending.isDone()
    while done == false:
        thread.yield()
        done = try pending.isDone()
    ..

    result := try pending.await()
    if strings.compare(result, "A") == false:
        strings.free(a, result)
        try pool.close()
        throw errors.failure("Async.read returned the wrong result")
    ..
    strings.free(a, result)

    failing := reader.new(none, failingSource)
    failed := try as.read(failing, 1)
    failedValue str, failedError error = failed.await()
    if failedError.ok():
        strings.free(a, failedValue)
        try pool.close()
        throw errors.failure("Async.read did not propagate the worker error")
    ..

    try pool.close()
..
