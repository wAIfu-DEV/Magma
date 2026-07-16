mod main

use "../std/allocator.mg" allocator
use "../std/heap.mg" heap
use "../std/io.mg" io
use "../std/time.mg" time
use "../std/writer.mg" writer

const ITERATIONS u64 = 1000000

benchAcquireAndCall(iterations u64) !u64:
    start := time.ticks()
    i u64 = 0
    while i < iterations:
        a := heap.allocator()
        block := try a.alloc(8)
        a.free(block)
        i = i + 1
    ..
    ret time.elapsedTicks(start)
..

benchSteady(iterations u64) !u64:
    a := heap.allocator()
    start := time.ticks()
    i u64 = 0
    while i < iterations:
        block := try a.alloc(8)
        a.free(block)
        i = i + 1
    ..
    ret time.elapsedTicks(start)
..

writeResult(out writer.Writer, label str, ticks u64) !void:
    try out.write(label)
    try out.writeUint64(ticks)
    try out.write(" ticks\n")
..

main() !void:
    out := io.stdoutUnbuffered()

    # Warm the allocator path before measuring.
    warm := try benchSteady(1000)

    try out.write("Allocator size: ")
    try out.writeUint64(sizeof allocator.Allocator)
    try out.write("B\n")

    try writeResult(out, "acquire+call: ", try benchAcquireAndCall(ITERATIONS))
    try writeResult(out, "steady:       ", try benchSteady(ITERATIONS))

    # Keep warmup results observable.
    if warm == 0:
        try out.write("")
    ..
..
