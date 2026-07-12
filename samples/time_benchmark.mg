mod main

# Compares the steady-state cost of time.runtime() and time.unixTimestamp().
# Build and run in release mode:
#   Magma.exe samples/time_benchmark.mg
#   clang.exe -O3 out.ll -o time_benchmark.exe
#   time_benchmark.exe

use "../std/allocator.mg" alc
use "../std/buffered.mg"  buffered
use "../std/cast.mg"      cast
use "../std/heap.mg"      heap
use "../std/io.mg"        io
use "../std/time.mg"      time
use "../std/writer.mg"    writer

pub main() !void:
    a alc.Allocator = heap.allocator()
    stdout buffered.Writer = try io.stdout(a)
    defer stdout.close()
    out writer.Writer = stdout.writer()

    # Warm up both paths. In particular, runtime() lazily caches the platform
    # tick frequency, and that one-time initialization should not skew it.
    warmup u64 = 0
    runtimeChecksum f64 = 0.0
    unixChecksum u64 = 0
    while warmup < 1000:
        runtimeChecksum = runtimeChecksum + time.runtime()
        unixChecksum = unixChecksum + time.unixTimestamp()
        warmup = warmup + 1
    ..

    iterations u64 = 5000000
    rounds u64 = 6
    round u64 = 0
    runtimeTicks u64 = 0
    unixTicks u64 = 0
    start u64 = 0
    i u64 = 0

    # Alternate which function runs first to reduce ordering and temperature
    # bias. Checksums keep every call observable to the optimizer.
    while round < rounds:
        if (round & 1) == 0:
            start = time.ticks()
            i = 0
            while i < iterations:
                runtimeChecksum = runtimeChecksum + time.runtime()
                i = i + 1
            ..
            runtimeTicks = runtimeTicks + time.elapsedTicks(start)

            start = time.ticks()
            i = 0
            while i < iterations:
                unixChecksum = unixChecksum + time.unixTimestamp()
                i = i + 1
            ..
            unixTicks = unixTicks + time.elapsedTicks(start)
        else:
            start = time.ticks()
            i = 0
            while i < iterations:
                unixChecksum = unixChecksum + time.unixTimestamp()
                i = i + 1
            ..
            unixTicks = unixTicks + time.elapsedTicks(start)

            start = time.ticks()
            i = 0
            while i < iterations:
                runtimeChecksum = runtimeChecksum + time.runtime()
                i = i + 1
            ..
            runtimeTicks = runtimeTicks + time.elapsedTicks(start)
        ..
        round = round + 1
    ..

    calls u64 = iterations * rounds
    runtimeNs f64 = (time.ticksToSecFloat(runtimeTicks) * 1000000000.0) / cast.utof(calls)
    unixNs f64 = (time.ticksToSecFloat(unixTicks) * 1000000000.0) / cast.utof(calls)

    try out.write("runtime():       ")
    try out.writeFloat64(runtimeNs, 3)
    try out.writeLn(" ns/call")
    try out.write("unixTimestamp(): ")
    try out.writeFloat64(unixNs, 3)
    try out.writeLn(" ns/call")

    if runtimeTicks > unixTicks:
        try out.writeLn("Slowest: runtime()")
    elif unixTicks > runtimeTicks:
        try out.writeLn("Slowest: unixTimestamp()")
    else:
        try out.writeLn("Result: tie at timer resolution")
    ..

    try out.write("Checksums: ")
    try out.writeFloat64(runtimeChecksum, 3)
    try out.write(" / ")
    try out.writeUint64(unixChecksum)
    try out.writeLn("")
..
