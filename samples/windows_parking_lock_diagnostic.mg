mod main
# Adaptive-lock benchmark against the platform mutex and spinlock.

use "std:atomic" as atomic
use "std:errors" as errors
use "std:io" as io
use "std:mutex" as mutex
use "std:slices" as slices
use "std:spinlock" as spinlock
use "std:thread" as thread
use "std:time" as time
use "std:writer" as writer
use "std:adaptive_spinlock" as adaptive_spinlock

const CREATION_ITERATIONS u64 = 1000000
const UNCONTENDED_ITERATIONS u64 = 5000000
const ITERATIONS u64 = 100000

@platform("windows")
use "std:win/thread_impl" as benchmark_thread_impl

@platform("linux", "android", "ios", "darwin", "freebsd", "netbsd", "openbsd")
use "std:unix/thread_impl" as benchmark_thread_impl

Candidate(state atomic.U32)
Context(lock adaptive_spinlock.AdaptiveSpinLock*, counter u64*, ready atomic.U64*, start atomic.U64*)
BaselineContext(lock mutex.Mutex*, counter u64*, ready atomic.U64*, start atomic.U64*)
SpinContext(lock spinlock.SpinLock*, counter u64*, ready atomic.U64*, start atomic.U64*)

compareExchange(value atomic.U32*, expected u32, desired u32) u32:
    unsafe:
        ret @llvm("cmpxchg_old", value, expected, desired, "acquire", "monotonic", 4, u32)
    ..
..

loadRelaxed(value atomic.U32*) u32:
    unsafe:
        ret @llvm("atomic_load", value, "monotonic", 4, u32)
    ..
..

storeRelease(value atomic.U32*, desired u32) void:
    unsafe:
        @llvm("atomic_store", value, desired, "release", 4, void)
    ..
..

exchangeAcquire(value atomic.U32*, desired u32) u32:
    unsafe:
        ret @llvm("atomic_rmw", value, desired, "xchg", "acquire", 4, u32)
    ..
..

cpuRelax() void:
    unsafe:
        @llvm("asm_sideeffect", "pause", void)
    ..
..

consume(value ptr) void:
    unsafe:
        @llvm("sideeffect", void)
    ..
..

tasLock(value Candidate*) void:
    loop exchangeAcquire(addrof value.state, 1) != 0:
        cpuRelax()
    ..
..

tasUnlock(value Candidate*) void:
    storeRelease(addrof value.state, 0)
..

ttasLock(value Candidate*) void:
    loop true:
        loop loadRelaxed(addrof value.state) != 0:
            cpuRelax()
        ..
        if compareExchange(addrof value.state, 0, 1) == 0:
            ret
        ..
    ..
..

ttasUnlock(value Candidate*) void:
    storeRelease(addrof value.state, 0)
..

backoffLock(value Candidate*) void:
    delay u64 = 1
    loop true:
        loop loadRelaxed(addrof value.state) != 0:
            cpuRelax()
        ..
        if compareExchange(addrof value.state, 0, 1) == 0:
            ret
        ..
        for i u64 = 0 to delay:
            cpuRelax()
        ..
        if delay < 64:
            delay = delay * 2
        ..
    ..
..

backoffUnlock(value Candidate*) void:
    storeRelease(addrof value.state, 0)
..

adaptiveLockSlow(value Candidate*) void:
    loop true:
        for spin u64 = 0 to 64:
            if loadRelaxed(addrof value.state) == 0:
                if compareExchange(addrof value.state, 0, 1) == 0:
                    ret
                ..
            ..
            cpuRelax()
        ..
        benchmark_thread_impl.yield()
    ..
..

adaptiveLock(value Candidate*) void:
    if compareExchange(addrof value.state, 0, 1) == 0:
        ret
    ..
    adaptiveLockSlow(value)
..

adaptiveUnlock(value Candidate*) void:
    storeRelease(addrof value.state, 0)
..

tasWorker(context Context*) u64:
    context.ready.fetchAdd(1)
    loop context.start.loadAcquire() == 0: thread.yield() ..
    for i u64 = 0 to ITERATIONS:
        tasLock(context.lock)
        *context.counter = *context.counter + 1
        tasUnlock(context.lock)
    ..
    ret 0
..

ttasWorker(context Context*) u64:
    context.ready.fetchAdd(1)
    loop context.start.loadAcquire() == 0: thread.yield() ..
    for i u64 = 0 to ITERATIONS:
        ttasLock(context.lock)
        *context.counter = *context.counter + 1
        ttasUnlock(context.lock)
    ..
    ret 0
..

backoffWorker(context Context*) u64:
    context.ready.fetchAdd(1)
    loop context.start.loadAcquire() == 0: thread.yield() ..
    for i u64 = 0 to ITERATIONS:
        backoffLock(context.lock)
        *context.counter = *context.counter + 1
        backoffUnlock(context.lock)
    ..
    ret 0
..

adaptiveWorker(context Context*) u64:
    context.ready.fetchAdd(1)
    loop context.start.loadAcquire() == 0: thread.yield() ..
    for i u64 = 0 to ITERATIONS:
        context.lock.lock()
        *context.counter = *context.counter + 1
        context.lock.unlock()
    ..
    ret 0
..

baselineLock(value mutex.Mutex*) !bool:
    try value.lock()
    ret true
..

baselineUnlock(value mutex.Mutex*) !bool:
    try value.unlock()
    ret true
..

baselineWorker(context BaselineContext*) u64:
    context.ready.fetchAdd(1)
    loop context.start.loadAcquire() == 0: thread.yield() ..
    for i u64 = 0 to ITERATIONS:
        acquired bool, acquireError error = baselineLock(context.lock)
        if acquireError.nok(): ret 1 ..
        *context.counter = *context.counter + 1
        released bool, releaseError error = baselineUnlock(context.lock)
        if releaseError.nok(): ret 1 ..
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

runCandidate(workerCount u64) !u64:
    guard := adaptive_spinlock.new()
    counter u64 = 0
    ready := atomic.newU64(0)
    start := atomic.newU64(0)
    contexts := array Context[24]
    workers := array thread.Thread[24]
    for i u64 = 0 to workerCount:
        contexts[i] = Context(lock=addrof guard, counter=addrof counter, ready=addrof ready, start=addrof start)
        workers[i] = try thread.new[Context](adaptiveWorker, addrof contexts[i])
    ..
    loop ready.loadAcquire() != workerCount:
        thread.yield()
    ..
    start.storeRelease(1)
    try thread.joinAll(slices.fromPtr(slices.toPtr(workers), workerCount))
    ret counter
..

runTas(workerCount u64) !u64:
    guard := adaptive_spinlock.new()
    counter u64 = 0
    ready := atomic.newU64(0)
    start := atomic.newU64(0)
    contexts := array Context[24]
    workers := array thread.Thread[24]
    for i u64 = 0 to workerCount:
        contexts[i] = Context(lock=addrof guard, counter=addrof counter, ready=addrof ready, start=addrof start)
        workers[i] = try thread.new[Context](tasWorker, addrof contexts[i])
    ..
    loop ready.loadAcquire() != workerCount: thread.yield() ..
    start.storeRelease(1)
    try thread.joinAll(slices.fromPtr(slices.toPtr(workers), workerCount))
    ret counter
..

runTtas(workerCount u64) !u64:
    guard := adaptive_spinlock.new()
    counter u64 = 0
    ready := atomic.newU64(0)
    start := atomic.newU64(0)
    contexts := array Context[24]
    workers := array thread.Thread[24]
    for i u64 = 0 to workerCount:
        contexts[i] = Context(lock=addrof guard, counter=addrof counter, ready=addrof ready, start=addrof start)
        workers[i] = try thread.new[Context](ttasWorker, addrof contexts[i])
    ..
    loop ready.loadAcquire() != workerCount: thread.yield() ..
    start.storeRelease(1)
    try thread.joinAll(slices.fromPtr(slices.toPtr(workers), workerCount))
    ret counter
..

runBackoff(workerCount u64) !u64:
    guard := adaptive_spinlock.new()
    counter u64 = 0
    ready := atomic.newU64(0)
    start := atomic.newU64(0)
    contexts := array Context[24]
    workers := array thread.Thread[24]
    for i u64 = 0 to workerCount:
        contexts[i] = Context(lock=addrof guard, counter=addrof counter, ready=addrof ready, start=addrof start)
        workers[i] = try thread.new[Context](backoffWorker, addrof contexts[i])
    ..
    loop ready.loadAcquire() != workerCount: thread.yield() ..
    start.storeRelease(1)
    try thread.joinAll(slices.fromPtr(slices.toPtr(workers), workerCount))
    ret counter
..

runSpin(workerCount u64) !u64:
    guard := spinlock.new()
    counter u64 = 0
    ready := atomic.newU64(0)
    start := atomic.newU64(0)
    contexts := array SpinContext[24]
    workers := array thread.Thread[24]
    for i u64 = 0 to workerCount:
        contexts[i] = SpinContext(lock=addrof guard, counter=addrof counter, ready=addrof ready, start=addrof start)
        workers[i] = try thread.new[SpinContext](spinWorker, addrof contexts[i])
    ..
    loop ready.loadAcquire() != workerCount: thread.yield() ..
    start.storeRelease(1)
    try thread.joinAll(slices.fromPtr(slices.toPtr(workers), workerCount))
    ret counter
..

runBaseline(workerCount u64) !u64:
    guard := try mutex.new()
    counter u64 = 0
    ready := atomic.newU64(0)
    start := atomic.newU64(0)
    contexts := array BaselineContext[24]
    workers := array thread.Thread[24]
    for i u64 = 0 to workerCount:
        contexts[i] = BaselineContext(lock=addrof guard, counter=addrof counter, ready=addrof ready, start=addrof start)
        workers[i] = try thread.new[BaselineContext](baselineWorker, addrof contexts[i])
    ..
    loop ready.loadAcquire() != workerCount: thread.yield() ..
    start.storeRelease(1)
    try thread.joinAll(slices.fromPtr(slices.toPtr(workers), workerCount))
    try guard.free()
    ret counter
..

writeValue(out writer.Writer*, name str, value u64) !void:
    try out.writeAll(" ")
    try out.writeAll(name)
    try out.writeAll("=")
    try out.writeUint64(value)
..

runContention(out writer.Writer*, workerCount u64) !void:
    expected := workerCount * ITERATIONS
    start := time.ticks()
    srwCount := try runBaseline(workerCount)
    srwNs := time.ticksToNs(time.elapsedTicks(start))
    start = time.ticks()
    adaptiveCount := try runCandidate(workerCount)
    adaptiveNs := time.ticksToNs(time.elapsedTicks(start))
    start = time.ticks()
    tasCount := try runTas(workerCount)
    tasNs := time.ticksToNs(time.elapsedTicks(start))
    start = time.ticks()
    ttasCount := try runTtas(workerCount)
    ttasNs := time.ticksToNs(time.elapsedTicks(start))
    start = time.ticks()
    backoffCount := try runBackoff(workerCount)
    backoffNs := time.ticksToNs(time.elapsedTicks(start))
    start = time.ticks()
    spinCount := try runSpin(workerCount)
    spinNs := time.ticksToNs(time.elapsedTicks(start))
    if srwCount != expected || adaptiveCount != expected || tasCount != expected || ttasCount != expected || backoffCount != expected || spinCount != expected:
        throw errors.failure("lock candidate lost protected increments")
    ..
    try out.writeAll("workers=")
    try out.writeUint64(workerCount)
    try writeValue(out, "mutex", srwNs)
    try writeValue(out, "adaptive", adaptiveNs)
    try writeValue(out, "tas", tasNs)
    try writeValue(out, "ttas", ttasNs)
    try writeValue(out, "backoff", backoffNs)
    try writeValue(out, "spinlock", spinNs)
    try out.writeAll("\n")
..

creation(out writer.Writer*) !void:
    start := time.ticks()
    for i u64 = 0 to CREATION_ITERATIONS:
        guard := try mutex.new()
        consume(addrof guard)
        try guard.free()
    ..
    srwNs := time.ticksToNs(time.elapsedTicks(start))
    start = time.ticks()
    for i u64 = 0 to CREATION_ITERATIONS:
        guard := adaptive_spinlock.new()
        consume(addrof guard)
    ..
    candidateNs := time.ticksToNs(time.elapsedTicks(start))
    start = time.ticks()
    for i u64 = 0 to CREATION_ITERATIONS:
        guard := spinlock.new()
        consume(addrof guard)
    ..
    spinNs := time.ticksToNs(time.elapsedTicks(start))
    try out.writeAll("creation")
    try writeValue(out, "mutex", srwNs)
    try writeValue(out, "candidate", candidateNs)
    try writeValue(out, "spinlock", spinNs)
    try out.writeAll("\n")
..

uncontended(out writer.Writer*) !void:
    srw := try mutex.new()
    start := time.ticks()
    for i u64 = 0 to UNCONTENDED_ITERATIONS:
        try srw.lock()
        try srw.unlock()
    ..
    srwNs := time.ticksToNs(time.elapsedTicks(start))
    try srw.free()

    guard := adaptive_spinlock.new()
    start = time.ticks()
    for i u64 = 0 to UNCONTENDED_ITERATIONS: guard.lock() guard.unlock() ..
    adaptiveNs := time.ticksToNs(time.elapsedTicks(start))
    raw := Candidate(state=atomic.newU32(0))
    start = time.ticks()
    for i u64 = 0 to UNCONTENDED_ITERATIONS: tasLock(addrof raw) tasUnlock(addrof raw) ..
    tasNs := time.ticksToNs(time.elapsedTicks(start))
    start = time.ticks()
    for i u64 = 0 to UNCONTENDED_ITERATIONS: ttasLock(addrof raw) ttasUnlock(addrof raw) ..
    ttasNs := time.ticksToNs(time.elapsedTicks(start))
    start = time.ticks()
    for i u64 = 0 to UNCONTENDED_ITERATIONS: backoffLock(addrof raw) backoffUnlock(addrof raw) ..
    backoffNs := time.ticksToNs(time.elapsedTicks(start))
    spin := spinlock.new()
    start = time.ticks()
    for i u64 = 0 to UNCONTENDED_ITERATIONS: spin.lock() spin.unlock() ..
    spinNs := time.ticksToNs(time.elapsedTicks(start))
    try out.writeAll("uncontended")
    try writeValue(out, "mutex", srwNs)
    try writeValue(out, "adaptive", adaptiveNs)
    try writeValue(out, "tas", tasNs)
    try writeValue(out, "ttas", ttasNs)
    try writeValue(out, "backoff", backoffNs)
    try writeValue(out, "spinlock", spinNs)
    try out.writeAll("\n")
..

pub main() !void:
    out := io.stdoutUnbuffered()
    try creation(addrof out)
    try uncontended(addrof out)
    try runContention(addrof out, 1)
    try runContention(addrof out, 2)
    try runContention(addrof out, 4)
    try runContention(addrof out, 8)
    try runContention(addrof out, 24)
..
