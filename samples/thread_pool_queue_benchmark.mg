mod main

use "../std/buffered.mg" buffered
use "../std/heap.mg" heap
use "../std/io.mg" io
use "../std/thread_pool.mg" thread_pool
use "../std/time.mg" time

const iterations u64 = 250000

noop(raw ptr) u64:
    ret 0
..

# Keeps the sole worker occupied so every measured submission enters the ring.
holdWorker(raw ptr) u64:
    time.sleep(1000)
    ret 0
..

benchmark(initialCapacity u64) !u64:
    a := heap.allocator()
    pool := try thread_pool.new(a, 1, initialCapacity)
    try pool.submit(holdWorker, none)
    time.sleep(10)
    start u64 = time.ticks()
    i u64 = 0
    while i < iterations:
        try pool.submit(noop, none)
        i = i + 1
    ..
    elapsed u64 = time.ticksToNs(time.elapsedTicks(start))
    try pool.close()
    ret elapsed / iterations
..

printResult(out buffered.Writer*, name str, value u64) !void:
    try out.write(name)
    try out.writeUint64(value)
    try out.writeLn(" ns/enqueue")
..

pub main() !void:
    a := heap.allocator()
    out := try io.stdout(a)
    defer out.close()

    presized u64 = try benchmark(iterations)
    growing u64 = try benchmark(8)

    try out.writeLn("250000 queued no-op tasks; one occupied worker")
    try printResult(addrof out, "pre-sized to task count:  ", presized)
    try printResult(addrof out, "growing, starts at 8:     ", growing)
..
