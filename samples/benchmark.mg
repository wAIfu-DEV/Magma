mod main

# A release-mode performance benchmark for Magma.
#
# Build with:
#   Magma.exe samples/benchmark.mg
#   clang.exe -O3 out.ll -o benchmark.exe
#
# This benchmark is intentionally non-interactive. It times CPU, sequential
# memory, unpredictable/random memory access, indirect calls, and allocation.
# Checksums make every workload observable and prevent dead-code elimination.

use "../std/allocator.mg" alc
use "../std/buffered.mg"  buffered
use "../std/cast.mg"      cast
use "../std/heap.mg"      heap
use "../std/io.mg"        io
use "../std/slices.mg"    slices
use "../std/writer.mg"    writer

@platform("windows")
ext ext_win32_QueryPerformanceCounter QueryPerformanceCounter(value i64*) i32

@platform("windows")
ext ext_win32_QueryPerformanceFrequency QueryPerformanceFrequency(value i64*) i32

Stepper(
    state ptr,
    stepFn (ptr, u64) u64,
)

loadU64(base ptr, index u64) u64:
    llvm "%p = getelementptr inbounds i64, ptr %base, i64 %index\n"
    llvm "%v = load i64, ptr %p, align 8\n"
    llvm "ret i64 %v\n"
..

storeU64(base ptr, index u64, value u64) void:
    llvm "%p = getelementptr inbounds i64, ptr %base, i64 %index\n"
    llvm "store i64 %value, ptr %p, align 8\n"
    llvm "ret void\n"
..

ticks() i64:
    value i64 = 0
    ext_win32_QueryPerformanceCounter(addrof value)
    ret value
..

secondsBetween(start i64, finish i64, frequency i64) f64:
    delta i64 = finish - start
    ret cast.itof(delta) / cast.itof(frequency)
..

# SplitMix64-style integer mixing. This is deliberately data-dependent and uses
# multiplication, shifts, and xor, making it a useful scalar optimizer test.
mix64(value u64) u64:
    x u64 = value
    x = (x ^ (x >> 30)) * 0xBF58476D1CE4E5B9
    x = (x ^ (x >> 27)) * 0x94D049BB133111EB
    ret x ^ (x >> 31)
..

cpuKernel(iterations u64, seed u64) u64:
    x u64 = seed
    sum u64 = 0
    i u64 = 0

    while i < iterations:
        x = x + 0x9E3779B97F4A7C15
        x = mix64(x)
        sum = sum ^ x
        i = i + 1
    ..

    ret sum
..

fillKernel(data ptr, count u64, seed u64) u64:
    x u64 = seed
    sum u64 = 0
    i u64 = 0

    while i < count:
        x = x + 0x9E3779B97F4A7C15
        x = mix64(x)
        storeU64(data, i, x)
        sum = sum + x
        i = i + 1
    ..

    ret sum
..

# Repeated full-buffer passes measure sequential loads/stores and give LLVM a
# loop it may vectorize. The result of each element depends on its old value.
sequentialMemoryKernel(data ptr, count u64, passes u64) u64:
    pass u64 = 0
    checksum u64 = 0
    i u64 = 0
    value u64 = 0

    while pass < passes:
        i = 0
        while i < count:
            value = loadU64(data, i)
            value = (value * 33) ^ (value >> 11) ^ i
            storeU64(data, i, value)
            checksum = checksum + value
            i = i + 1
        ..
        pass = pass + 1
    ..

    ret checksum
..

# The next address depends on the value just loaded. This defeats simple
# prefetching and measures cache misses plus dependent-load latency.
randomAccessKernel(data ptr, iterations u64, seed u64) u64:
    # The dataset contains 2^23 elements, so masking keeps indexes in range.
    mask u64 = 8388607
    index u64 = seed & mask
    checksum u64 = seed
    i u64 = 0

    while i < iterations:
        index = (loadU64(data, index) ^ checksum) & mask
        checksum = checksum + loadU64(data, index)
        i = i + 1
    ..

    ret checksum ^ index
..

dispatchStep(state ptr, value u64) u64:
    old u64 = loadU64(state, 0)
    next u64 = (old * 1664525) + 1013904223 + value
    storeU64(state, 0, next)
    ret next ^ (next >> 17)
..

# The call is made through a function-pointer field. This tests whether the
# compiler can remove abstraction overhead when the concrete target is known.
dispatchKernel(stepper Stepper*, iterations u64) u64:
    checksum u64 = 0
    i u64 = 0

    while i < iterations:
        checksum = checksum + stepper.stepFn(stepper.state, i)
        i = i + 1
    ..

    ret checksum
..

# Allocation is measured separately because it primarily evaluates the Magma
# allocator abstraction and Windows heap, not pure generated arithmetic.
allocationKernel(a alc.Allocator, count u64) !u64:
    checksum u64 = 0
    i u64 = 0
    block u8* = cast.utop(0)

    while i < count:
        block = try a.alloc(64)
        storeU64(block, 0, i)
        storeU64(block, 7, i ^ 0xA5A5A5A5)
        checksum = checksum + loadU64(block, 0) + loadU64(block, 7)
        a.free(block)
        i = i + 1
    ..

    ret checksum
..

printResult(out writer.Writer*, name str, elapsed f64, checksum u64) !void:
    try out.write(name)
    try out.write(": ")
    try out.writeFloat64(elapsed, 6)
    try out.write(" s  checksum=")
    try out.writeUint64(checksum)
    try out.writeLn("")
..

pub main(args str[]) !void:
    a alc.Allocator = heap.allocator()
    stdout buffered.Writer = try io.stdout(a)
    defer stdout.close()
    out writer.Writer = stdout.writer()

    frequency i64 = 0
    if ext_win32_QueryPerformanceFrequency(addrof frequency) == 0:
        try out.writeLn("QueryPerformanceFrequency failed")
        ret
    ..

    try out.writeLn("Magma performance benchmark (release builds only)")
    try out.writeLn("Dataset: 64 MiB; times exclude allocation/setup unless named")

    # Make the seed depend on a runtime value so the whole benchmark cannot be
    # evaluated at compile time. The harness supplies one argument.
    # Local constants are used because global constant declarations are not yet
    # supported by the frontend.
    elementCount u64 = 8388608
    seed u64 = 0x123456789ABCDEF0 + slices.count(args)
    data u8* = try a.alloc(elementCount * sizeof u64)
    defer a.free(data)

    start i64 = ticks()
    checksum u64 = cpuKernel(100000000, seed)
    finish i64 = ticks()
    try printResult(addrof out, "scalar integer", secondsBetween(start, finish, frequency), checksum)

    start = ticks()
    checksum = fillKernel(data, elementCount, checksum)
    finish = ticks()
    try printResult(addrof out, "fill 64 MiB", secondsBetween(start, finish, frequency), checksum)

    start = ticks()
    checksum = sequentialMemoryKernel(data, elementCount, 8)
    finish = ticks()
    try printResult(addrof out, "sequential memory", secondsBetween(start, finish, frequency), checksum)

    start = ticks()
    checksum = randomAccessKernel(data, 25000000, checksum)
    finish = ticks()
    try printResult(addrof out, "random memory", secondsBetween(start, finish, frequency), checksum)

    dispatchState u64 = checksum
    stepper Stepper
    stepper.state = addrof dispatchState
    stepper.stepFn = dispatchStep

    start = ticks()
    checksum = dispatchKernel(addrof stepper, 100000000)
    finish = ticks()
    try printResult(addrof out, "function dispatch", secondsBetween(start, finish, frequency), checksum)

    start = ticks()
    checksum = try allocationKernel(a, 1000000)
    finish = ticks()
    try printResult(addrof out, "allocation churn", secondsBetween(start, finish, frequency), checksum)

    try out.writeLn("Done. Compare medians from at least 5 runs on an idle machine.")
..
