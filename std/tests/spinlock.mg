mod main

use "std:atomic" atomic
use "std:errors" errors
use "std:spinlock" spinlock
use "std:thread" thread

const workerCount u64 = 4
const incrementsPerWorker u64 = 10000

Context(
    lock spinlock.SpinLock*
    value u64*
    ready atomic.U64*
    start atomic.U64*
)

worker(context Context*) u64:
    context.ready.fetchAdd(1)
    while context.start.load() == 0:
        thread.yield()
    ..

    i u64 = 0
    while i < incrementsPerWorker:
        context.lock.lock()
        *context.value = *context.value + 1
        context.lock.unlock()
        i = i + 1
    ..
    ret 0
..

pub main() !void:
    lock := spinlock.new()
    value u64 = 0
    ready := atomic.newU64(0)
    start := atomic.newU64(0)
    contexts := array Context[4]
    threads := array thread.Thread[4]

    i u64 = 0
    while i < workerCount:
        contexts[i] = Context(lock=addrof lock, value=addrof value, ready=addrof ready, start=addrof start)
        threads[i] = try thread.new[Context](worker, addrof contexts[i])
        i = i + 1
    ..
    while ready.load() != workerCount:
        thread.yield()
    ..
    start.store(1)
    try thread.joinAll(threads)

    if value != workerCount * incrementsPerWorker:
        throw errors.failure("spinlock did not provide mutual exclusion")
    ..
..
