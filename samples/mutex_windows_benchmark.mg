mod main
# Windows benchmark comparing std:mutex's SRWLOCK backend with std:spinlock.

use "std:atomic" atomic
use "std:errors" errors
use "std:io" io
use "std:mutex" mutex
use "std:slices" slices
use "std:spinlock" spinlock
use "std:thread" thread
use "std:time" time
use "std:writer" writer

const CREATION_ITERATIONS u64 = 1000000
const UNCONTENDED_ITERATIONS u64 = 5000000
const ITERATIONS u64 = 100000
const MAX_WORKERS u64 = 24

consumeMutex(value ptr) void:
    unsafe:
        llvm "  call void asm sideeffect \"\", \"r,~{memory}\"(ptr %value)\n"
        llvm "  ret void\n"
    ..
..

MutexContext(lock mutex.Mutex*, counter u64*, ready atomic.U64*, start atomic.U64*)
SpinContext(lock spinlock.SpinLock*, counter u64*, ready atomic.U64*, start atomic.U64*)

mutexWorker(context MutexContext*) u64:
    context.ready.fetchAdd(1)
    loop context.start.loadAcquire() == 0: thread.yield() ..
    for i u64 = 0 to ITERATIONS:
        locked bool, lockError error = mutexLock(context.lock)
        if lockError.nok(): ret 1 ..
        *context.counter = *context.counter + 1
        unlocked bool, unlockError error = mutexUnlock(context.lock)
        if unlockError.nok(): ret 1 ..
    ..
    ret 0
..

spinWorker(context SpinContext*) u64:
    context.ready.fetchAdd(1)
    loop context.start.loadAcquire() == 0: thread.yield() ..
    for i u64 = 0 to ITERATIONS:
        context.lock.lock()
        *context.counter = *context.counter + 1
        context.lock.unlock()
    ..
    ret 0
..

mutexLock(value mutex.Mutex*) !bool:
    try value.lock()
    ret true
..

mutexUnlock(value mutex.Mutex*) !bool:
    try value.unlock()
    ret true
..

mutexCreation() !void:
    for i u64 = 0 to CREATION_ITERATIONS:
        lock := try mutex.new()
        consumeMutex(addrof lock)
        try lock.free()
    ..
..

spinCreation() void:
    for i u64 = 0 to CREATION_ITERATIONS:
        lock := spinlock.new()
        consumeMutex(addrof lock)
    ..
..

mutexUncontended() !void:
    lock := try mutex.new()
    for i u64 = 0 to UNCONTENDED_ITERATIONS:
        try lock.lock()
        try lock.unlock()
    ..
    try lock.free()
..

spinUncontended() void:
    lock := spinlock.new()
    for i u64 = 0 to UNCONTENDED_ITERATIONS:
        lock.lock()
        lock.unlock()
    ..
..

mutexContended(workerCount u64) !u64:
    lock := try mutex.new()
    counter u64 = 0
    ready := atomic.newU64(0)
    start := atomic.newU64(0)
    contexts := array MutexContext[24]
    workers := array thread.Thread[24]
    for i u64 = 0 to workerCount:
        contexts[i] = MutexContext(lock=addrof lock, counter=addrof counter, ready=addrof ready, start=addrof start)
        workers[i] = try thread.new[MutexContext](mutexWorker, addrof contexts[i])
    ..
    loop ready.loadAcquire() != workerCount: thread.yield() ..
    start.storeRelease(1)
    try thread.joinAll(slices.fromPtr(slices.toPtr(workers), workerCount))
    try lock.free()
    ret counter
..

spinContended(workerCount u64) !u64:
    lock := spinlock.new()
    counter u64 = 0
    ready := atomic.newU64(0)
    start := atomic.newU64(0)
    contexts := array SpinContext[24]
    workers := array thread.Thread[24]
    for i u64 = 0 to workerCount:
        contexts[i] = SpinContext(lock=addrof lock, counter=addrof counter, ready=addrof ready, start=addrof start)
        workers[i] = try thread.new[SpinContext](spinWorker, addrof contexts[i])
    ..
    loop ready.loadAcquire() != workerCount: thread.yield() ..
    start.storeRelease(1)
    try thread.joinAll(slices.fromPtr(slices.toPtr(workers), workerCount))
    ret counter
..

writeLifecycle(out writer.Writer*, name str, creation u64, uncontended u64) !void:
    try out.writeAll(name)
    try out.writeAll(" creation_ns=")
    try out.writeUint64(creation)
    try out.writeAll(" uncontended_ns=")
    try out.writeUint64(uncontended)
    try out.writeAll("\n")
..

runContention(out writer.Writer*, workerCount u64) !void:
    expected := workerCount * ITERATIONS
    start := time.ticks()
    mutexCount := try mutexContended(workerCount)
    mutexNs := time.ticksToNs(time.elapsedTicks(start))
    start = time.ticks()
    spinCount := try spinContended(workerCount)
    spinNs := time.ticksToNs(time.elapsedTicks(start))
    if mutexCount != expected || spinCount != expected:
        throw errors.failure("Windows lock benchmark lost protected increments")
    ..
    try out.writeAll("workers=")
    try out.writeUint64(workerCount)
    try out.writeAll(" mutex_ns=")
    try out.writeUint64(mutexNs)
    try out.writeAll(" spinlock_ns=")
    try out.writeUint64(spinNs)
    try out.writeAll("\n")
..

pub main() !void:
    start := time.ticks()
    try mutexCreation()
    mutexCreate := time.ticksToNs(time.elapsedTicks(start))
    start = time.ticks()
    spinCreation()
    spinCreate := time.ticksToNs(time.elapsedTicks(start))

    start = time.ticks()
    try mutexUncontended()
    mutexSingle := time.ticksToNs(time.elapsedTicks(start))
    start = time.ticks()
    spinUncontended()
    spinSingle := time.ticksToNs(time.elapsedTicks(start))

    out := io.stdoutUnbuffered()
    try out.writeAll("creation_iterations=1000000 uncontended_iterations=5000000 increments_per_worker=100000\n")
    try writeLifecycle(addrof out, "mutex_srw", mutexCreate, mutexSingle)
    try writeLifecycle(addrof out, "spinlock", spinCreate, spinSingle)
    try runContention(addrof out, 2)
    try runContention(addrof out, 4)
    try runContention(addrof out, 8)
    try runContention(addrof out, MAX_WORKERS)
..
