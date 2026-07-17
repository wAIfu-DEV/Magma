mod main

use "../std/allocator.mg" allocator
use "../std/buffered.mg" buffered
use "../std/cast.mg" cast
use "../std/errors.mg" errors
use "../std/fake_alloc.mg" fake_alloc
use "../std/heap.mg" heap
use "../std/io.mg" io
use "../std/thread.mg" thread
use "../std/thread_pool.mg" thread_pool
use "../std/time.mg" time

const totalOperations u64 = 40000000
const maximumWorkers u64 = 32
Context(
    iterations u64
    sampleCount u64
    ready u64*
    start u64*
    failures u64
    samples error*
)

atomicAdd(target u64*, value u64) u64:
    llvm "  %previous = atomicrmw add ptr %target, i64 %value acq_rel, align 8\n"
    llvm "  ret i64 %previous\n"
..

atomicLoad(target u64*) u64:
    llvm "  %value = load atomic i64, ptr %target acquire, align 8\n"
    llvm "  ret i64 %value\n"
..

atomicStore(target u64*, value u64) void:
    llvm "  store atomic i64 %value, ptr %target release, align 8\n"
    llvm "  ret void\n"
..

traceSlotCount() u64:
    llvm "  %count = call i64 @magma.error.trace.capacity()\n"
    llvm "  ret i64 %count\n"
..

contextAt(base Context*, index u64) Context*:
    ret cast.utop(cast.ptou(base) + (index * sizeof Context))
..

sampleAt(base error*, index u64) error*:
    ret cast.utop(cast.ptou(base) + (index * sizeof error))
..

worker(raw ptr) u64:
    context Context* = raw
    failing allocator.Allocator = fake_alloc.allocator()
    atomicAdd(context.ready, 1)
    while atomicLoad(context.start) == 0:
        thread.yield()
    ..

    failures u64 = 0
    i u64 = 0
    while i < context.iterations:
        block, failure := failing.alloc(8)
        if errors.isError(failure):
            failures = failures + 1
            if i + context.sampleCount >= context.iterations:
                sampleIndex u64 = i - (context.iterations - context.sampleCount)
                *sampleAt(context.samples, sampleIndex) = failure
            ..
        else:
            failing.free(block)
        ..
        i = i + 1
    ..
    context.failures = failures
    ret 0
..

traceWasTruncated(failure error) bool:
    cursor := errors.trace(failure)
    while cursor.isEmpty() == false:
        cursor = cursor.next()
    ..
    ret cursor.isTruncated()
..

printResult(out buffered.Writer*, workers u64, elapsedNs u64, failures u64, truncated u64) !void:
    try out.writeUint64(workers)
    try out.write(" workers: ")
    try out.writeUint64(elapsedNs / totalOperations)
    try out.write(" ns/error, ")
    if elapsedNs != 0:
        try out.writeUint64((totalOperations * 1000000000) / elapsedNs)
    else:
        try out.writeUint64(0)
    ..
    try out.write(" errors/s, failures=")
    try out.writeUint64(failures)
    try out.write(", trace truncation=")
    percentUnits u64 = (truncated * 10000000) / failures
    try out.writeUint64(percentUnits / 100000)
    try out.write(".")
    remainder u64 = percentUnits % 100000
    if remainder < 10000:
        try out.write("0")
    ..
    if remainder < 1000:
        try out.write("0")
    ..
    if remainder < 100:
        try out.write("0")
    ..
    if remainder < 10:
        try out.write("0")
    ..
    try out.writeUint64(remainder)
    try out.write("% (")
    try out.writeUint64(truncated)
    try out.write("/")
    try out.writeUint64(failures)
    try out.write(" workload errors)")
    try out.writeLn("")
..

benchmark(out buffered.Writer*, workers u64) !void:
    backing allocator.Allocator = heap.allocator()
    pool := try thread_pool.new(backing, workers, workers, 100)
    base Context* = try backing.allocT[Context](workers)
    iterations u64 = totalOperations / workers
    sampleCount u64 = traceSlotCount()
    if sampleCount > iterations:
        sampleCount = iterations
    ..
    samples error* = try backing.allocT[error](workers * sampleCount)
    ready u64 = 0
    start u64 = 0

    i u64 = 0
    while i < workers:
        context Context* = contextAt(base, i)
        context.iterations = iterations
        context.sampleCount = sampleCount
        context.ready = addrof ready
        context.start = addrof start
        context.failures = 0
        context.samples = sampleAt(samples, i * sampleCount)
        try pool.submit(worker, context)
        i = i + 1
    ..
    while atomicLoad(addrof ready) != workers:
        thread.yield()
    ..

    began u64 = time.ticks()
    atomicStore(addrof start, 1)
    try pool.wait()
    elapsedNs u64 = time.ticksToNs(time.elapsedTicks(began))

    failures u64 = 0
    truncated u64 = 0
    i = 0
    while i < workers:
        context Context* = contextAt(base, i)
        failures = failures + context.failures
        sample u64 = 0
        while sample < sampleCount:
            if traceWasTruncated(*sampleAt(context.samples, sample)):
                truncated = truncated + 1
            ..
            sample = sample + 1
        ..
        i = i + 1
    ..
    try pool.close()
    backing.free(samples)
    backing.free(base)
    retained u64 = (workers * sampleCount) - truncated
    workloadTruncated u64 = failures - retained
    try printResult(out, workers, elapsedNs, failures, workloadTruncated)
..

pub main() !void:
    backing allocator.Allocator = heap.allocator()
    out := try io.stdout(backing)
    defer out.close()

    try out.writeLn("Fake allocator error-trace benchmark")
    try out.writeLn("40,000,000 failed allocations per row; two trace pushes per allocation")
    try out.writeLn("Trace truncation is reported over every failed allocation in each row")
    workers u64 = 1
    while workers <= maximumWorkers:
        try benchmark(addrof out, workers)
        workers = workers * 2
    ..
..
