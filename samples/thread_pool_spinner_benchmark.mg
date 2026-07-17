mod main

use "../std/allocator.mg" allocator
use "../std/buffered.mg" buffered
use "../std/heap.mg" heap
use "../std/io.mg" io
use "../std/thread_pool.mg" thread_pool
use "../std/time.mg" time

const iterations u64 = 500
const spinCount u64 = 4096

Context(
    start u64
    elapsed u64
)

entry(raw ptr) u64:
    entered u64 = time.ticks()
    context Context* = raw
    context.elapsed = entered - context.start
    ret 0
..

delay(us u64) void:
    if us == 0:
        ret
    ..
    start u64 = time.ticks()
    target u64 = time.usToTicks(us)
    while time.elapsedTicks(start) < target:
    ..
..

benchmark(pool thread_pool.ThreadPool, gapUs u64) !u64:
    total u64 = 0
    i u64 = 0
    while i < iterations:
        delay(gapUs)
        context Context
        context.start = time.ticks()
        try pool.submit(entry, addrof context)
        try pool.wait()
        total = total + context.elapsed
        i = i + 1
    ..
    ret time.ticksToNs(total) / iterations
..

printResult(out buffered.Writer*, gapUs u64, normal u64, spinning u64) !void:
    try out.writeUint64(gapUs)
    try out.write(" us: normal=")
    try out.writeUint64(normal)
    try out.write(" ns, spinner=")
    try out.writeUint64(spinning)
    try out.write(" ns, spinner/normal=")
    try out.writeUint64((spinning * 100) / normal)
    try out.writeLn("%")
..

pub main() !void:
    a allocator.Allocator = heap.allocator()
    out := try io.stdout(a)
    defer out.close()
    normal := try thread_pool.new(a, 1, 8)
    spinning := try thread_pool.newSpinning(a, 1, 8, spinCount)

    warmup Context
    try normal.submit(entry, addrof warmup)
    try normal.wait()
    try spinning.submit(entry, addrof warmup)
    try spinning.wait()

    try out.writeLn("Submission to first callback instruction; 500 iterations; one worker")
    try out.writeLn("Spinner budget: 4096 pause instructions")
    normalNs u64 = try benchmark(normal, 0)
    spinningNs u64 = try benchmark(spinning, 0)
    try printResult(addrof out, 0, normalNs, spinningNs)
    normalNs = try benchmark(normal, 10)
    spinningNs = try benchmark(spinning, 10)
    try printResult(addrof out, 10, normalNs, spinningNs)
    normalNs = try benchmark(normal, 25)
    spinningNs = try benchmark(spinning, 25)
    try printResult(addrof out, 25, normalNs, spinningNs)
    normalNs = try benchmark(normal, 50)
    spinningNs = try benchmark(spinning, 50)
    try printResult(addrof out, 50, normalNs, spinningNs)
    normalNs = try benchmark(normal, 100)
    spinningNs = try benchmark(spinning, 100)
    try printResult(addrof out, 100, normalNs, spinningNs)
    normalNs = try benchmark(normal, 1000)
    spinningNs = try benchmark(spinning, 1000)
    try printResult(addrof out, 1000, normalNs, spinningNs)

    try normal.close()
    try spinning.close()
..
