mod main

use "../std/buffered.mg" buffered
use "../std/heap.mg" heap
use "../std/io.mg" io
use "../std/thread.mg" thread
use "../std/thread_pool.mg" thread_pool
use "../std/time.mg" time

const iterations u64 = 2000
const wakeIterations u64 = 100

noop(raw ptr) u64:
    ret 0
..

benchmarkThreads() !u64:
    start u64 = time.ticks()
    i u64 = 0
    while i < iterations:
        worker := try thread.new[ptr](noop, none)
        try worker.join()
        i = i + 1
    ..
    ret time.ticksToNs(time.elapsedTicks(start)) / iterations
..

# Waiting after every task ensures the worker has normally gone back to sleep;
# this measures submit-to-completion wake latency rather than queue throughput.
benchmarkPool() !u64:
    a := heap.allocator()
    pool := try thread_pool.new(a, 1, 1)
    total u64 = 0
    i u64 = 0
    while i < wakeIterations:
        # Let the worker reach its OS wait before measuring its wake-up.
        time.sleep(1)
        start u64 = time.ticks()
        try pool.submit(noop, none)
        try pool.wait()
        total = total + time.elapsedTicks(start)
        i = i + 1
    ..
    elapsed u64 = time.ticksToNs(total) / wakeIterations
    try pool.shutdown()
    ret elapsed
..

printResult(out buffered.Writer*, name str, value u64) !void:
    try out.write(name)
    try out.writeUint64(value)
    try out.writeLn(" ns/op")
..

pub main() !void:
    allocator := heap.allocator()
    out := try io.stdout(allocator)
    defer out.close()

    # Warm up code paths and native runtime state before measuring.
    warmup := try thread_pool.new(allocator, 1, 1)
    try warmup.submit(noop, none)
    try warmup.wait()
    try warmup.shutdown()

    threadNs u64 = try benchmarkThreads()
    poolNs u64 = try benchmarkPool()

    try out.writeLn("Single no-op task, one worker")
    try out.writeLn("Thread creation: 2000 iterations; sleeping wake: 100 iterations")
    try printResult(addrof out, "new Thread + join:       ", threadNs)
    try printResult(addrof out, "ThreadPool:              ", poolNs)
..
