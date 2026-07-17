mod main

use "../std/allocator.mg" allocator
use "../std/buffered.mg" buffered
use "../std/heap.mg" heap
use "../std/io.mg" io
use "../std/thread.mg" thread
use "../std/thread_pool.mg" thread_pool
use "../std/time.mg" time

const iterations u64 = 2000

Context(
    start u64
    elapsed u64
)

# The timestamp is deliberately the first operation in both callbacks.
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
        context Context
        context.start = time.ticks()
        try pool.submit(poolEntry, addrof context)
        try pool.wait()
        total = total + context.elapsed
        i = i + 1
    ..
    ret time.ticksToNs(total) / iterations
..

printResult(out buffered.Writer*, name str, value u64) !void:
    try out.write(name)
    try out.writeUint64(value)
    try out.writeLn(" ns/op")
..

printRatio(out buffered.Writer*, basisPoints u64) !void:
    try out.write("Pool/Thread:    ")
    try out.writeUint64(basisPoints / 100)
    try out.write(".")
    remainder u64 = basisPoints % 100
    if remainder < 10:
        try out.write("0")
    ..
    try out.writeUint64(remainder)
    try out.writeLn("%")
..

pub main() !void:
    a allocator.Allocator = heap.allocator()
    out := try io.stdout(a)
    defer out.close()

    # Pool construction and its first task are outside the measurement.
    pool := try thread_pool.new(a, 1, 8)
    warmup Context
    warmup.start = time.ticks()
    try pool.submit(poolEntry, addrof warmup)
    try pool.wait()

    threadNs u64 = try benchmarkThreads()
    poolNs u64 = try benchmarkPool(pool)
    try pool.close()

    try out.writeLn("Creation/submission to first callback instruction")
    try out.writeLn("2000 sequential iterations; one pre-created warm pool worker")
    try printResult(addrof out, "native Thread:  ", threadNs)
    try printResult(addrof out, "warm ThreadPool:", poolNs)

    if threadNs != 0:
        try printRatio(addrof out, (poolNs * 10000) / threadNs)
    ..
..
