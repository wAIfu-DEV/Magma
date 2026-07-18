mod main

use "../std/allocator.mg" allocator
use "../std/buffered.mg" buffered
use "../std/future.mg" future
use "../std/heap.mg" heap
use "../std/io.mg" io
use "../std/thread.mg" thread
use "../std/thread_pool.mg" thread_pool
use "../std/time.mg" time

const iterations u64 = 500

Context(
    start u64
    elapsed u64
)

FutureContext(
    start u64
)

threadEntry(context Context*) u64:
    entered u64 = time.ticks()
    context.elapsed = entered - context.start
    ret 0
..

poolEntry(raw ptr) u64:
    entered u64 = time.ticks()
    context Context* = raw
    context.elapsed = entered - context.start
    ret 0
..

futureEntry(context FutureContext*) !u64:
    entered u64 = time.ticks()
    ret entered - context.start
..

startFuture(a allocator.Allocator, pool thread_pool.ThreadPool, start u64) !$future.Future[u64]:
    ret try future.new[u64, FutureContext](a, pool, futureEntry, FutureContext(start=start))
..

benchmarkThreads() !u64:
    total u64 = 0
    i u64 = 0
    while i < iterations:
        context Context
        context.start = time.ticks()
        worker := try thread.new[Context](threadEntry, addrof context)
        try worker.join()
        total = total + context.elapsed
        i = i + 1
    ..
    ret time.ticksToNs(total) / iterations
..

benchmarkPool(pool thread_pool.ThreadPool) !u64:
    total u64 = 0
    i u64 = 0
    while i < iterations:
        time.sleep(1)
        context Context
        context.start = time.ticks()
        try pool.submit(poolEntry, addrof context)
        try pool.wait()
        total = total + context.elapsed
        i = i + 1
    ..
    ret time.ticksToNs(total) / iterations
..

benchmarkFutures(a allocator.Allocator, pool thread_pool.ThreadPool) !u64:
    total u64 = 0
    i u64 = 0
    while i < iterations:
        time.sleep(1)
        start u64 = time.ticks()
        pending := try startFuture(a, pool, start)
        total = total + try pending.await()
        try pool.wait()
        i = i + 1
    ..
    ret time.ticksToNs(total) / iterations
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

    pool := try thread_pool.new(a, 1, 1, 8, 1)
    warmup Context
    warmup.start = time.ticks()
    try pool.submit(poolEntry, addrof warmup)
    try pool.wait()

    threadNs u64 = try benchmarkThreads()
    poolNs u64 = try benchmarkPool(pool)
    futureNs u64 = try benchmarkFutures(a, pool)
    try pool.close()

    try out.writeLn("Creation/submission to first callback instruction")
    try out.writeLn("500 sequential iterations; one warm pool worker, parked before pooled samples")
    try printResult(addrof out, "new Thread: ", threadNs)
    try printResult(addrof out, "ThreadPool: ", poolNs)
    try printResult(addrof out, "Future:     ", futureNs)
..
