mod main

use "../std/allocator.mg" allocator
use "../std/buffered.mg" buffered
use "../std/cast.mg" cast
use "../std/cpu.mg" cpu
use "../std/future.mg" future
use "../std/heap.mg" heap
use "../std/io.mg" io
use "../std/thread.mg" thread
use "../std/thread_pool.mg" thread_pool
use "../std/time.mg" time

const workIterations u64 = 20000

# Measures a growing burst of concurrently pending Futures. The timer starts
# immediately before Future allocation/submission and stops after every Future
# has been awaited and the pool has completed the batch. Consequently:
#
# - `submit` includes allocating each Future, initializing its wait state, and
#   queueing it, but excludes task execution because tasks wait on `gate`;
# - `total` includes submission, pool growth, task dispatch and execution,
#   Future wake/await handling, and draining the pool;
# - `ns/future` is total wall-clock time divided by the batch size. It is a
#   concurrent throughput figure, not the latency of an individual Future;
# - `workers` is the active worker count sampled after the entire batch has
#   been submitted and before the gate opens.
#
# The shared gate deliberately keeps early tasks occupied while later Futures
# are submitted. This creates enough simultaneous demand to exercise the
# pool's min-to-max growth path instead of benchmarking a mostly sequential
# stream of short tasks.

Context(
    gate u64*
    value u64
)

atomicLoad(target u64*) u64:
    llvm "  %value = load atomic i64, ptr %target acquire, align 8\n"
    llvm "  ret i64 %value\n"
..

atomicStore(target u64*, value u64) void:
    llvm "  store atomic i64 %value, ptr %target release, align 8\n"
    llvm "  ret void\n"
..

run(context Context*) !u64:
    while atomicLoad(context.gate) == 0:
        thread.yield()
    ..
    value u64 = context.value + 1
    i u64 = 0
    while i < workIterations:
        value = (value * 1664525) + 1013904223
        i = i + 1
    ..
    ret value
..

futureAt(base future.Future[u64]*, index u64) future.Future[u64]*:
    ret cast.utop(cast.ptou(base) + (index * sizeof future.Future[u64]))
..

benchmark(out buffered.Writer*, a allocator.Allocator, pool thread_pool.ThreadPool, count u64) !void:
    pending future.Future[u64]* = try a.allocT[future.Future[u64]](count)
    gate u64 = 0
    start u64 = time.ticks()
    i u64 = 0
    while i < count:
        *futureAt(pending, i) = try future.new[u64, Context](a, pool, run, Context(gate=addrof gate, value=i))
        i = i + 1
    ..
    submitted u64 = time.ticks()

    try pool.state.lock.lock()
    workers u64 = pool.state.activeWorkers
    try pool.state.lock.unlock()
    atomicStore(addrof gate, 1)

    checksum u64 = 0
    i = 0
    while i < count:
        checksum = checksum + try futureAt(pending, i).await()
        i = i + 1
    ..
    try pool.wait()
    finished u64 = time.ticks()
    a.free(pending)

    submitNs u64 = time.ticksToNs(submitted - start)
    totalNs u64 = time.ticksToNs(finished - start)
    try out.writeUint64(count)
    try out.write(" futures: workers=")
    try out.writeUint64(workers)
    try out.write(", submit=")
    try out.writeUint64(submitNs / 1000000)
    try out.write(" ms, total=")
    try out.writeUint64(totalNs / 1000000)
    try out.write(" ms, ")
    try out.writeUint64(totalNs / count)
    try out.write(" ns/future, checksum=")
    try out.writeUint64(checksum)
    try out.writeLn("")
..

pub main() !void:
    a allocator.Allocator = heap.allocator()
    out := try io.stdout(a)
    defer out.close()

    maxWorkers u64 = cpu.coreCount()
    if maxWorkers > 16:
        maxWorkers = 16
    ..
    pool := try thread_pool.new(a, 1, maxWorkers, 256, 1)

    try out.write("Async growing-workload benchmark; min=1, max=")
    try out.writeUint64(maxWorkers)
    try out.write(", work iterations=")
    try out.writeUint64(workIterations)
    try out.writeLn("")

    count u64 = 64
    while count <= 16384:
        try benchmark(addrof out, a, pool, count)
        count = count * 4
    ..
    try pool.close()
..
