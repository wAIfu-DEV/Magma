mod main
# Windows benchmark for the former SRWLOCK backend versus std:mutex's
# three-state WaitOnAddress backend. Build and run this sample on Windows.

use "std:atomic" atomic
use "std:errors" errors
use "std:io" io
use "std:mutex" mutex
use "std:slices" slices
use "std:thread" thread
use "std:time" time
use "std:writer" writer

const CREATION_ITERATIONS u64 = 1000000
const UNCONTENDED_ITERATIONS u64 = 5000000
const ITERATIONS u64 = 100000
const MAX_WORKERS u64 = 24

# SRWLOCK_INIT is one zero-initialized pointer and requires no destruction.
SrwMutex(state ptr)

ext ext_win32_AcquireSRWLockExclusive AcquireSRWLockExclusive(value SrwMutex*) void
ext ext_win32_ReleaseSRWLockExclusive ReleaseSRWLockExclusive(value SrwMutex*) void

consumeMutex(value ptr) void:
    unsafe:
        llvm "  call void asm sideeffect \"\", \"r,~{memory}\"(ptr %value)\n"
        llvm "  ret void\n"
    ..
..

SrwContext(lock SrwMutex*, counter u64*, ready atomic.U64*, start atomic.U64*)
CandidateContext(lock mutex.Mutex*, counter u64*, ready atomic.U64*, start atomic.U64*)

srwWorker(context SrwContext*) u64:
    context.ready.fetchAdd(1)
    loop context.start.loadAcquire() == 0: thread.yield() ..
    for i u64 = 0 to ITERATIONS:
        ext_win32_AcquireSRWLockExclusive(context.lock)
        *context.counter = *context.counter + 1
        ext_win32_ReleaseSRWLockExclusive(context.lock)
    ..
    ret 0
..

candidateWorker(context CandidateContext*) u64:
    context.ready.fetchAdd(1)
    loop context.start.loadAcquire() == 0: thread.yield() ..
    for i u64 = 0 to ITERATIONS:
        locked bool, lockError error = candidateLock(context.lock)
        if lockError.nok(): ret 1 ..
        *context.counter = *context.counter + 1
        unlocked bool, unlockError error = candidateUnlock(context.lock)
        if unlockError.nok(): ret 1 ..
    ..
    ret 0
..

candidateLock(value mutex.Mutex*) !bool:
    try value.lock()
    ret true
..

candidateUnlock(value mutex.Mutex*) !bool:
    try value.unlock()
    ret true
..

srwCreation() void:
    for i u64 = 0 to CREATION_ITERATIONS:
        lock := SrwMutex(state=none)
        consumeMutex(addrof lock)
    ..
..

candidateCreation() !void:
    for i u64 = 0 to CREATION_ITERATIONS:
        lock := try mutex.new()
        consumeMutex(addrof lock)
        try lock.free()
    ..
..

srwUncontended() void:
    lock := SrwMutex(state=none)
    for i u64 = 0 to UNCONTENDED_ITERATIONS:
        ext_win32_AcquireSRWLockExclusive(addrof lock)
        ext_win32_ReleaseSRWLockExclusive(addrof lock)
    ..
..

candidateUncontended() !void:
    lock := try mutex.new()
    for i u64 = 0 to UNCONTENDED_ITERATIONS:
        try lock.lock()
        try lock.unlock()
    ..
    try lock.free()
..

srwContended(workerCount u64) !u64:
    lock := SrwMutex(state=none)
    counter u64 = 0
    ready := atomic.newU64(0)
    start := atomic.newU64(0)
    contexts := array SrwContext[24]
    workers := array thread.Thread[24]
    for i u64 = 0 to workerCount:
        contexts[i] = SrwContext(lock=addrof lock, counter=addrof counter, ready=addrof ready, start=addrof start)
        workers[i] = try thread.new[SrwContext](srwWorker, addrof contexts[i])
    ..
    loop ready.loadAcquire() != workerCount: thread.yield() ..
    start.storeRelease(1)
    try thread.joinAll(slices.fromPtr(slices.toPtr(workers), workerCount))
    ret counter
..

candidateContended(workerCount u64) !u64:
    lock := try mutex.new()
    counter u64 = 0
    ready := atomic.newU64(0)
    start := atomic.newU64(0)
    contexts := array CandidateContext[24]
    workers := array thread.Thread[24]
    for i u64 = 0 to workerCount:
        contexts[i] = CandidateContext(lock=addrof lock, counter=addrof counter, ready=addrof ready, start=addrof start)
        workers[i] = try thread.new[CandidateContext](candidateWorker, addrof contexts[i])
    ..
    loop ready.loadAcquire() != workerCount: thread.yield() ..
    start.storeRelease(1)
    try thread.joinAll(slices.fromPtr(slices.toPtr(workers), workerCount))
    try lock.free()
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
    srwCount := try srwContended(workerCount)
    srwNs := time.ticksToNs(time.elapsedTicks(start))
    start = time.ticks()
    candidateCount := try candidateContended(workerCount)
    candidateNs := time.ticksToNs(time.elapsedTicks(start))
    if srwCount != expected || candidateCount != expected:
        throw errors.failure("Windows mutex candidate lost protected increments")
    ..
    try out.writeAll("workers=")
    try out.writeUint64(workerCount)
    try out.writeAll(" srw_ns=")
    try out.writeUint64(srwNs)
    try out.writeAll(" three_state_ns=")
    try out.writeUint64(candidateNs)
    try out.writeAll("\n")
..

pub main() !void:
    start := time.ticks()
    srwCreation()
    srwCreate := time.ticksToNs(time.elapsedTicks(start))
    start = time.ticks()
    try candidateCreation()
    candidateCreate := time.ticksToNs(time.elapsedTicks(start))

    start = time.ticks()
    srwUncontended()
    srwSingle := time.ticksToNs(time.elapsedTicks(start))
    start = time.ticks()
    try candidateUncontended()
    candidateSingle := time.ticksToNs(time.elapsedTicks(start))

    out := io.stdoutUnbuffered()
    try out.writeAll("creation_iterations=1000000 uncontended_iterations=5000000 increments_per_worker=100000\n")
    try writeLifecycle(addrof out, "srw", srwCreate, srwSingle)
    try writeLifecycle(addrof out, "three_state", candidateCreate, candidateSingle)
    try runContention(addrof out, 2)
    try runContention(addrof out, 4)
    try runContention(addrof out, 8)
    try runContention(addrof out, MAX_WORKERS)
..
