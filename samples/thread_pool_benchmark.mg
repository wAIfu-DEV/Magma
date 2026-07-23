mod main

use "../std/allocator.mg" allocator
use "../std/cpu.mg" cpu
use "../std/errors.mg" errors
use "../std/heap.mg" heap
use "../std/io.mg" io
use "../std/thread_pool.mg" thread_pool
use "../std/time.mg" time

TASKS u64 = 1000000
ROUNDS u64 = 5

increment(raw ptr) u64:
    llvm "  %ignored = atomicrmw add ptr %raw, i64 1 monotonic, align 8\n"
    llvm "  ret i64 0\n"
..

makePool(a allocator.Allocator, spinning bool, workers u64) !$thread_pool.ThreadPool:
    if spinning:
        ret try thread_pool.new(a, workers, workers, TASKS, 4096)
    ..
    ret try thread_pool.new(a, workers, workers, TASKS, 0)
..

run(a allocator.Allocator, spinning bool, workers u64) !u64:
    counter u64 = 0
    pool := try makePool(a, spinning, workers)

    start u64 = time.ticks()
    i u64 = 0
    while i < TASKS:
        try pool.submit(increment, addrof counter)
        i = i + 1
    ..
    try pool.close()
    elapsed u64 = time.elapsedUs(start)
    if counter != TASKS:
        throw errors.failure("thread-pool benchmark lost tasks")
    ..
    ret elapsed
..

pub main() !void:
    a allocator.Allocator = heap.allocator()
    workers u64 = cpu.coreCount()

    # Warm both implementations before collecting samples.
    mutexWarmup u64 = try run(a, false, workers)
    spinWarmup u64 = try run(a, true, workers)

    mutexTotal u64 = 0
    spinTotal u64 = 0
    round u64 = 0
    while round < ROUNDS:
        # Alternate order to reduce bias from temperature and background load.
        if round % 2 == 0:
            mutexTotal = mutexTotal + try run(a, false, workers)
            spinTotal = spinTotal + try run(a, true, workers)
        else:
            spinTotal = spinTotal + try run(a, true, workers)
            mutexTotal = mutexTotal + try run(a, false, workers)
        ..
        round = round + 1
    ..

    mutexAverage u64 = mutexTotal / ROUNDS
    spinAverage u64 = spinTotal / ROUNDS
    out := io.stdoutUnbuffered()
    try out.writeAll("High-task-count thread-pool benchmark\nTasks per round: ")
    try out.writeUint64(TASKS)
    try out.writeAll("\nWorkers: ")
    try out.writeUint64(workers)
    try out.writeAll("\nRounds: ")
    try out.writeUint64(ROUNDS)
    try out.writeAll("\nMutex average (us): ")
    try out.writeUint64(mutexAverage)
    try out.writeAll("\nSpinLock average (us): ")
    try out.writeUint64(spinAverage)
    try out.writeAll("\nFaster: ")
    if spinAverage < mutexAverage:
        try out.writeAll("SpinLock\n")
    elif mutexAverage < spinAverage:
        try out.writeAll("Mutex\n")
    else:
        try out.writeAll("tie\n")
    ..
..
