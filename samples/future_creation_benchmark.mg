mod main

use "../std/allocator.mg" allocator
use "../std/buffered.mg" buffered
use "../std/future.mg" future
use "../std/heap.mg" heap
use "../std/io.mg" io
use "../std/thread.mg" thread
use "../std/thread_pool.mg" thread_pool
use "../std/time.mg" time

const iterations u64 = 2000
const parkedIterations u64 = 200

ThreadContext(
    start u64
    elapsed u64
)

FutureContext(
    start u64
)

# The timestamp is deliberately the first operation in each entry callback.
threadEntry(context ThreadContext*) u64:
    entered u64 = time.ticks()
    context.elapsed = entered - context.start
    ret 0
..

futureEntry(context FutureContext*) !u64:
    entered u64 = time.ticks()
    ret entered - context.start
..

# Models an ordinary public async wrapper: its arguments are packaged into a
# private heap context before being handed to Future.new.
startFuture(a allocator.Allocator, pool thread_pool.ThreadPool, start u64) !$future.Future[u64]:
    context := FutureContext(start=start)
    ret try future.new[u64, FutureContext](a, pool, futureEntry, context)
..

benchmarkThreads() !u64:
    total u64 = 0
    i u64 = 0
    while i < iterations:
        context ThreadContext
        context.start = time.ticks()
        worker := try thread.new[ThreadContext](threadEntry, addrof context)
        try worker.join()
        total = total + context.elapsed
        i = i + 1
    ..
    ret time.ticksToNs(total) / iterations
..

benchmarkFutures(a allocator.Allocator, pool thread_pool.ThreadPool) !u64:
    total u64 = 0
    i u64 = 0
    while i < parkedIterations:
        # Keep this outside the measured interval. It gives the worker enough
        # time to enter its native wait, so every sample includes a wake syscall.
        time.sleep(1)
        start u64 = time.ticks()
        pending := try startFuture(a, pool, start)
        total = total + try pending.await()

        # Await observes Future completion before ThreadPool has necessarily
        # finished its bookkeeping. Waiting keeps iterations independent.
        try pool.wait()
        i = i + 1
    ..
    ret time.ticksToNs(total) / parkedIterations
..

printResult(out buffered.Writer*, name str, value u64) !void:
    try out.write(name)
    try out.writeUint64(value)
    try out.writeLn(" ns/op")
..

pub main() !void:
    a allocator.Allocator = heap.allocator()
    out := try io.stdout(a)
    defer out.close()

    # Pool construction and initial worker startup are outside the measurement.
    pool := try thread_pool.new(a, 1, 8)
    warmupStart u64 = time.ticks()
    warmup := try startFuture(a, pool, warmupStart)
    try warmup.await()
    try pool.wait()

    threadNs u64 = try benchmarkThreads()
    futureNs u64 = try benchmarkFutures(a, pool)
    try pool.close()

    try out.writeLn("Creation/submission to first callback instruction")
    try out.writeLn("Thread: 2000 iterations; Future: 200 parked-worker iterations")
    try printResult(addrof out, "native Thread:  ", threadNs)
    try printResult(addrof out, "Future:         ", futureNs)

    if threadNs != 0:
        try out.write("Future/Thread:  ")
        try out.writeUint64((futureNs * 100) / threadNs)
        try out.writeLn("%")
    ..
..
