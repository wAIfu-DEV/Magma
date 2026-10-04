mod main
# Common lock failure modes: longer critical sections, scheduler yields while
# owned, blocking owners, oversubscription, and acquisition fairness.

use "std:adaptive_spinlock" as adaptive_spinlock
use "std:atomic" as atomic
use "std:errors" as errors
use "std:io" as io
use "std:locker" as locker
use "std:mutex" as mutex
use "std:slices" as slices
use "std:spinlock" as spinlock
use "std:thread" as thread
use "std:time" as time
use "std:writer" as writer

const MODE_WORK u64 = 0
const MODE_YIELD u64 = 1
const MODE_SLEEP u64 = 2
const MODE_FAIRNESS u64 = 3

Context(
    guard locker.Locker*
    total u64*
    counts u64*
    index u64
    iterations u64
    work u64
    target u64
    mode u64
    ready atomic.U64*
    start atomic.U64*
    failed atomic.U64*
)

Result(wallNs u64, cpuNs u64, minimum u64, maximum u64, total u64)

cpuRelax() void:
    unsafe:
        @llvm("asm_sideeffect", "pause", void)
    ..
..

lockResult(value locker.Locker*) !bool:
    try value.lock()
    ret true
..

unlockResult(value locker.Locker*) !bool:
    try value.unlock()
    ret true
..

incrementCount(counts u64*, index u64) void:
    unsafe:
        counts[index] = counts[index] + 1
    ..
..

worker(context Context*) u64:
    context.ready.fetchAdd(1)
    loop context.start.loadAcquire() == 0: thread.yield() ..
    completed u64 = 0
    loop context.mode == MODE_FAIRNESS || completed < context.iterations:
        locked bool, lockError error = lockResult(context.guard)
        if lockError.nok(): context.failed.store(1) ret 1 ..
        if context.mode == MODE_FAIRNESS && *context.total >= context.target:
            unlocked bool, unlockError error = unlockResult(context.guard)
            if unlockError.nok(): context.failed.store(1) ret 1 ..
            break
        ..
        if context.mode == MODE_WORK:
            for i u64 = 0 to context.work: cpuRelax() ..
        elif context.mode == MODE_YIELD:
            thread.yield()
        elif context.mode == MODE_SLEEP:
            time.sleep(1)
        ..
        *context.total = *context.total + 1
        incrementCount(context.counts, context.index)
        unlocked bool, unlockError error = unlockResult(context.guard)
        if unlockError.nok(): context.failed.store(1) ret 1 ..
        completed = completed + 1
    ..
    ret 0
..

run(guard locker.Locker*, workerCount u64, iterations u64, work u64, target u64, mode u64) !Result:
    total u64 = 0
    counts := array u64[24]
    ready := atomic.newU64(0)
    startFlag := atomic.newU64(0)
    failed := atomic.newU64(0)
    contexts := array Context[24]
    workers := array thread.Thread[24]
    for i u64 = 0 to workerCount:
        counts[i] = 0
        contexts[i] = Context(guard=guard, total=addrof total, counts=slices.toPtr(counts), index=i, iterations=iterations, work=work, target=target, mode=mode, ready=addrof ready, start=addrof startFlag, failed=addrof failed)
        workers[i] = try thread.new[Context](worker, addrof contexts[i])
    ..
    loop ready.loadAcquire() != workerCount: thread.yield() ..
    cpuStart := time.processCpuTimeNs()
    wallStart := time.ticks()
    startFlag.storeRelease(1)
    try thread.joinAll(slices.fromPtr(slices.toPtr(workers), workerCount))
    wallNs := time.ticksToNs(time.elapsedTicks(wallStart))
    cpuNs := time.processCpuTimeNs() - cpuStart
    if failed.load() != 0: throw errors.failure("lock pathology worker failed") ..
    expected := workerCount * iterations
    if mode == MODE_FAIRNESS: expected = target ..
    if total != expected: throw errors.failure("lock pathology benchmark lost protected work") ..
    minimum := counts[0]
    maximum := counts[0]
    for i u64 = 1 to workerCount:
        if counts[i] < minimum: minimum = counts[i] ..
        if counts[i] > maximum: maximum = counts[i] ..
    ..
    ret Result(wallNs=wallNs, cpuNs=cpuNs, minimum=minimum, maximum=maximum, total=total)
..

writeResult(out writer.Writer*, workload str, name str, result Result) !void:
    try out.writeAll(workload)
    try out.writeAll(" lock=")
    try out.writeAll(name)
    try out.writeAll(" wall_ns=")
    try out.writeUint64(result.wallNs)
    try out.writeAll(" cpu_ns=")
    try out.writeUint64(result.cpuNs)
    try out.writeAll(" min=")
    try out.writeUint64(result.minimum)
    try out.writeAll(" max=")
    try out.writeUint64(result.maximum)
    try out.writeAll("\n")
..

runMutex(out writer.Writer*, workload str, workers u64, iterations u64, work u64, target u64, mode u64) !void:
    backend := try mutex.new()
    view := backend.locker()
    result := try run(addrof view, workers, iterations, work, target, mode)
    try backend.free()
    try writeResult(out, workload, "mutex", result)
..

runAdaptive(out writer.Writer*, workload str, workers u64, iterations u64, work u64, target u64, mode u64) !void:
    backend := adaptive_spinlock.new()
    view := backend.locker()
    result := try run(addrof view, workers, iterations, work, target, mode)
    try writeResult(out, workload, "adaptive", result)
..

runSpin(out writer.Writer*, workload str, workers u64, iterations u64, work u64, target u64, mode u64) !void:
    backend := spinlock.new()
    view := backend.locker()
    result := try run(addrof view, workers, iterations, work, target, mode)
    try writeResult(out, workload, "yielding_spin", result)
..

runAll(out writer.Writer*, workload str, workers u64, iterations u64, work u64, target u64, mode u64) !void:
    try runMutex(out, workload, workers, iterations, work, target, mode)
    try runAdaptive(out, workload, workers, iterations, work, target, mode)
    try runSpin(out, workload, workers, iterations, work, target, mode)
..

pub main() !void:
    out := io.stdoutUnbuffered()
    try runAll(addrof out, "work64_8t", 8, 20000, 64, 0, MODE_WORK)
    try runAll(addrof out, "work512_8t", 8, 5000, 512, 0, MODE_WORK)
    try runAll(addrof out, "work64_24t", 24, 5000, 64, 0, MODE_WORK)
    try runAll(addrof out, "owner_yield_8t", 8, 5000, 0, 0, MODE_YIELD)
    try runAll(addrof out, "owner_sleep1ms_4t", 4, 25, 0, 0, MODE_SLEEP)
    try runAll(addrof out, "fairness_12t", 12, 0, 0, 500000, MODE_FAIRNESS)
..
