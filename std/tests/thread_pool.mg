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
    try pool.close()
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

    zeroSpin thread_pool.ThreadPool, spinErr error = thread_pool.newSpinning(a, 1, 1, 0)
    if errors.code(spinErr) != 2:
        throw errors.failure("spinning thread pool accepted zero spin count")
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
    try pool.close()
    if value != 33:
        throw errors.failure("thread pool shutdown did not drain queued work")
    ..

    # shutdown clears the handle and rejects a second release.
    stopped bool, shutdownErr error = shutdownResult(addrof pool)
    if errors.code(shutdownErr) != 2 || pool.state != none:
        throw errors.failure("thread pool allowed repeated shutdown")
    ..

    spinningValue u64 = 0
    spinning := try thread_pool.newSpinning(a, 1, 8, 4096)
    spinRound u64 = 0
    while spinRound < 100:
        spinIndex u64 = 0
        while spinIndex < 8:
            try spinning.submit(increment, addrof spinningValue)
            spinIndex = spinIndex + 1
        ..
        try spinning.wait()
        spinRound = spinRound + 1
    ..
    try spinning.close()
    if spinningValue != 800:
        throw errors.failure("spinning thread pool did not execute every task")
    ..

    # A tiny initial ring must grow enough to accept a large submission burst.
    growingValue u64 = 0
    growing := try thread_pool.new(a, 1, 1)
    growingIndex u64 = 0
    while growingIndex < 10000:
        try growing.submit(increment, addrof growingValue)
        growingIndex = growingIndex + 1
    ..
    try growing.close()
    if growingValue != 10000:
        throw errors.failure("thread pool queue did not grow correctly")
    ..

..
