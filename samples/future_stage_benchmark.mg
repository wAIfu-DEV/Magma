mod main

use "../std/allocator.mg" allocator
use "../std/buffered.mg" buffered
use "../std/future.mg" future
use "../std/heap.mg" heap
use "../std/io.mg" io
use "../std/thread_pool.mg" thread_pool
use "../std/time.mg" time

const iterations u64 = 10000

Context(
    value u64
)

entry(context Context*) !u64:
    ret context.value
..

start(a allocator.Allocator, pool thread_pool.ThreadPool, value u64) !$future.Future[u64]:
    ret try future.new[u64, Context](a, pool, entry, Context(value=value))
..

printAverage(out buffered.Writer*, name str, ticks u64, samples u64) !void:
    try out.write(name)
    try out.writeUint64(time.ticksToNs(ticks) / samples)
    try out.writeLn(" ns/op")
..

pub main() !void:
    a allocator.Allocator = heap.allocator()
    out := try io.stdout(a)
    defer out.close()
    pool := try thread_pool.new(a, 1, 8)

    warmup := try start(a, pool, 0)
    try warmup.await()
    try pool.wait()
    future.resetCreationTiming()

    wallStart u64 = time.ticks()
    i u64 = 0
    while i < iterations:
        pending := try start(a, pool, i)
        try pending.await()
        try pool.wait()
        i = i + 1
    ..
    wallElapsed u64 = time.ticks() - wallStart
    timing future.CreationTiming = future.creationTiming()
    try pool.close()

    try out.writeLn("Future.new internal phase timing")
    try out.write("samples:        ")
    try out.writeUint64(timing.samples)
    try out.writeLn("")
    try printAverage(addrof out, "allocation:     ", timing.allocationTicks, timing.samples)
    try printAverage(addrof out, "initialization: ", timing.initializationTicks, timing.samples)
    try printAverage(addrof out, "waiter setup:   ", timing.waiterTicks, timing.samples)
    try printAverage(addrof out, "submission:     ", timing.submissionTicks, timing.samples)
    totalTicks u64 = timing.allocationTicks + timing.initializationTicks + timing.waiterTicks + timing.submissionTicks
    try printAverage(addrof out, "measured total: ", totalTicks, timing.samples)
    try printAverage(addrof out, "full loop:      ", wallElapsed, iterations)
..
