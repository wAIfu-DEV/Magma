mod main

use "../allocator.mg" allocator
use "../cast.mg" cast
use "../errors.mg" errors
use "../heap.mg" heap
use "../thread_pool.mg" thread_pool

increment(raw ptr) u64:
    value u64* = raw
    *value = *value + 1
    ret 0
..

shutdownResult(pool thread_pool.ThreadPool*) !bool:
    try pool.shutdown()
    ret true
..

expectInvalidSizes(a allocator.Allocator) !void:
    zeroWorkers thread_pool.ThreadPool, workerErr error = thread_pool.new(a, 0, 1)
    if errors.code(workerErr) != 2:
        throw errors.failure("thread pool accepted zero workers")
    ..

    zeroCapacity thread_pool.ThreadPool, capacityErr error = thread_pool.new(a, 1, 0)
    if errors.code(capacityErr) != 2:
        throw errors.failure("thread pool accepted zero queue capacity")
    ..
..

pub main() !void:
    a allocator.Allocator = heap.allocator()
    try expectInvalidSizes(a)

    value u64 = 0
    pool := try thread_pool.new(a, 1, 8)

    # Exercise more tasks than the ring capacity across several idle cycles.
    round u64 = 0
    while round < 4:
        i u64 = 0
        while i < 8:
            try pool.submit(increment, addrof value)
            i = i + 1
        ..
        try pool.wait()
        round = round + 1
    ..
    if value != 32:
        throw errors.failure("thread pool did not execute every task")
    ..

    # Waiting at an already-idle point must return immediately.
    try pool.wait()
    try pool.submit(increment, addrof value)
    try pool.shutdown()
    if value != 33:
        throw errors.failure("thread pool shutdown did not drain queued work")
    ..

    # shutdown clears the handle and rejects a second release.
    stopped bool, shutdownErr error = shutdownResult(addrof pool)
    if errors.code(shutdownErr) != 2 || pool.state != none:
        throw errors.failure("thread pool allowed repeated shutdown")
    ..
..
