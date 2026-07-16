mod main

use "../std/buffered.mg" buffered
use "../std/heap.mg" heap
use "../std/io.mg" io
use "../std/thread_pool.mg" thread_pool
use "../std/time.mg" time

const idleMilliseconds u64 = 2000
const workerCount u64 = 4

# Returns CPU utilization in basis points: 10000 means 100.00% of one core.
measureSleep() u64:
    wallStart u64 = time.ticks()
    cpuStart u64 = time.processCpuTimeNs()
    time.sleep(idleMilliseconds)
    cpuElapsed u64 = time.processCpuTimeNs() - cpuStart
    wallElapsed u64 = time.ticksToNs(time.elapsedTicks(wallStart))
    if wallElapsed == 0:
        ret 0
    ..
    ret (cpuElapsed * 10000) / wallElapsed
..

measureBusyControl() u64:
    wallStart u64 = time.ticks()
    cpuStart u64 = time.processCpuTimeNs()
    while time.elapsedMs(wallStart) < 500:
        # Re-reading the monotonic clock keeps one core intentionally active.
        time.ticks()
    ..
    cpuElapsed u64 = time.processCpuTimeNs() - cpuStart
    wallElapsed u64 = time.ticksToNs(time.elapsedTicks(wallStart))
    ret (cpuElapsed * 10000) / wallElapsed
..

measurePool() !u64:
    a := heap.allocator()
    pool := try thread_pool.new(a, workerCount, 64)
    # Ensure every worker has reached its blocking wait before sampling.
    time.sleep(100)
    load u64 = measureSleep()
    try pool.shutdown()
    ret load
..

printLoad(out buffered.Writer*, name str, basisPoints u64) !void:
    try out.write(name)
    try out.writeUint64(basisPoints / 100)
    try out.write(".")
    remainder u64 = basisPoints % 100
    if remainder < 10:
        try out.write("0")
    ..
    try out.writeUint64(remainder)
    try out.writeLn("% of one CPU core")
..

pub main() !void:
    allocator := heap.allocator()
    out := try io.stdout(allocator)
    defer out.close()

    baseline u64 = measureSleep()
    poolLoad u64 = try measurePool()
    busyControl u64 = measureBusyControl()

    try out.writeLn("Idle CPU load, four workers, two-second sample")
    try printLoad(addrof out, "baseline sleep:  ", baseline)
    try printLoad(addrof out, "ThreadPool:      ", poolLoad)
    try printLoad(addrof out, "busy control:    ", busyControl)
..
