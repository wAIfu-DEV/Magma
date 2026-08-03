# `std/thread_pool`

`ThreadPool` sends tasks through a growing ring buffer to a dynamically sized
set of native threads. It keeps a configurable minimum number of workers, grows when queued work has
consumed every available worker, and retires burst workers when the queue
drains. Its baseline workers briefly spin before parking when `spinCount` is
nonzero.

## Construction

```magma
normal := try thread_pool.new(a, 2, 8, 256, 0)
spinning := try thread_pool.new(a, 2, 8, 256, 4096)
```

`new(a, minWorkers, maxWorkers, queueCapacity, spinCount)` creates a pool with
the specified worker limits, initial queue capacity, and spin budget. It initially creates
`minWorkers` workers and grows toward `maxWorkers` as concurrent demand requires. An idle worker
registers itself as sleeping and waits using the platform generation-wait
backend. On Windows this uses `WaitOnAddress`; Unix platforms use the standard
library's mutex/condition fallback.

When `spinCount` is nonzero and the queue becomes empty, the remaining baseline workers check the atomic
work generation up to `spinCount` times. An LLVM `pause` instruction is issued
between checks. If submission changes the generation during that interval, the
worker checks the queue without entering the operating-system wait. If the
budget expires, the worker parks exactly like a normal pool worker.

Both worker limits and `queueCapacity` must be greater than zero, and
`maxWorkers` must not be less than `minWorkers`. A zero `spinCount` disables
spinning.

`newDefault(a) !$ThreadPool` uses the logical core count as its baseline,
permits worker growth, starts with a 256-task queue, and selects a small
core-count-derived spin budget.

Worker bookkeeping starts at `minWorkers` slots and doubles only when those
slots are occupied, capped by `maxWorkers`. Contexts are allocated separately,
so this growth does not relocate context pointers held by running workers.

When the queue fills, submission doubles its capacity and linearizes queued
tasks into the new ring in FIFO order. Growth is amortized O(1) and occurs while
holding the pool spin lock. Allocation failure or integer capacity exhaustion is
returned without modifying the existing queue.

`spinCount` is an iteration budget, not a duration. Its duration depends on the
processor, clock state, compiler target, and contention. Measure it on the
target hardware rather than treating a particular count as a time unit.

## Submission and lifetime

`submit(entry, context)` queues `entry(context)`. The context is borrowed and
must remain valid until that task completes. Submission grows a full queue
rather than blocking the submitting thread.

`wait()` blocks until all pending work is complete. `close()` first drains
pending work, stops and joins all workers, and releases the pool. It consumes
the pool.

`executor()` returns a borrowed `std/executor.Executor` view. This permits APIs
that accept a type-erased scheduler to submit typed contexts to the pool. The
pool must remain alive and open until all work submitted through the view has
completed; freeing the borrowed view does not close the pool.

```magma
pool := try thread_pool.new(a, 2, 8, 256, 0)
try pool.submit(doWork, context)
try pool.wait()
try pool.close()
```

## Spinner trade-offs

Spinning exchanges idle CPU time for lower dispatch latency. While a worker is
inside its spin phase, submission only advances the atomic work generation; it
does not need to wake a parked native thread. If a worker has already parked,
submission uses the normal wake path.

Workers needed for a burst retire when the queue drains, leaving only the
baseline workers to use the configured spin phase. A conservative spin budget is
still advisable for mostly idle workloads.

Use the normal pool for mostly idle or latency-insensitive work. Consider the
spinning pool when tasks arrive frequently enough that avoiding native wakeups
is worth the extra CPU usage.

See `samples/thread_pool_benchmark.mg` for the maintained pool example.
