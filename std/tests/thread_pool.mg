mod main

use "../allocator.mg" allocator
use "../cast.mg" cast
use "../errors.mg" errors
use "../heap.mg" heap
use "../thread_pool.mg" thread_pool
use "../thread.mg" thread
use "../time.mg" time

ScaleContext(
    ready u64*
    release u64*
)

atomicAdd(target u64*, value u64) void:
    llvm "  %ignored = atomicrmw add ptr %target, i64 %value acq_rel, align 8\n"
    llvm "  ret void\n"
..

atomicLoad(target u64*) u64:
    llvm "  %value = load atomic i64, ptr %target acquire, align 8\n"
    llvm "  ret i64 %value\n"
..

atomicStore(target u64*, value u64) void:
    llvm "  store atomic i64 %value, ptr %target release, align 8\n"
    llvm "  ret void\n"
..

occupy(raw ptr) u64:
    context ScaleContext* = raw
    atomicAdd(context.ready, 1)
    while atomicLoad(context.release) == 0:
        thread.yield()
    ..
    ret 0
..

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
    zeroWorkers thread_pool.ThreadPool, workerErr error = thread_pool.new(a, 0, 1, 1, 1)
    if errors.code(workerErr) != 2:
        throw errors.failure("thread pool accepted zero workers")
    ..

    invertedWorkers thread_pool.ThreadPool, limitErr error = thread_pool.new(a, 2, 1, 1, 1)
    if errors.code(limitErr) != 2:
        throw errors.failure("thread pool accepted a maximum below its minimum")
    ..

    zeroCapacity thread_pool.ThreadPool, capacityErr error = thread_pool.new(a, 1, 1, 0, 1)
    if errors.code(capacityErr) != 2:
        throw errors.failure("thread pool accepted zero queue capacity")
    ..

    zeroSpin thread_pool.ThreadPool, spinErr error = thread_pool.new(a, 1, 1, 1, 0)
    if errors.code(spinErr) != 2:
        throw errors.failure("spinning thread pool accepted zero spin count")
    ..
..

pub main() !void:
    a allocator.Allocator = heap.allocator()
    try expectInvalidSizes(a)

    value u64 = 0
    pool := try thread_pool.new(a, 1, 1, 8, 1)

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
    spinning := try thread_pool.newSpinning(a, 1, 1, 8, 4096)
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
    growing := try thread_pool.new(a, 1, 1, 1, 1)
    growingIndex u64 = 0
    while growingIndex < 10000:
        try growing.submit(increment, addrof growingValue)
        growingIndex = growingIndex + 1
    ..
    try growing.close()
    if growingValue != 10000:
        throw errors.failure("thread pool queue did not grow correctly")
    ..

    # The configured maximum is a ceiling. A pool starts at its minimum,
    # grows while queued work has consumed every available worker, then returns
    # then returns to that minimum after the burst drains.
    ready u64 = 0
    release u64 = 0
    scaleContext := ScaleContext(ready=addrof ready, release=addrof release)
    scaling := try thread_pool.new(a, 2, 4, 4, 1)
    scaleIndex u64 = 0
    while scaleIndex < 4:
        try scaling.submit(occupy, addrof scaleContext)
        scaleIndex = scaleIndex + 1
    ..
    deadline u64 = time.ticks() + time.msToTicks(2000)
    while atomicLoad(addrof ready) != 4 && time.ticks() < deadline:
        thread.yield()
    ..
    if atomicLoad(addrof ready) != 4:
        atomicStore(addrof release, 1)
        try scaling.close()
        throw errors.failure("thread pool did not grow when all workers were busy")
    ..
    atomicStore(addrof release, 1)
    try scaling.wait()
    shrinkDeadline u64 = time.ticks() + time.msToTicks(2000)
    active u64 = 4
    while active != 2 && time.ticks() < shrinkDeadline:
        try scaling.state.lock.lock()
        active = scaling.state.activeWorkers
        try scaling.state.lock.unlock()
        thread.yield()
    ..
    if active != 2:
        try scaling.close()
        throw errors.failure("thread pool did not shrink after its burst drained")
    ..
    try scaling.close()

..
