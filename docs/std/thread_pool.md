# `std/thread_pool`

`ThreadPool` keeps a fixed number of native threads alive and sends tasks to
them through a growing ring buffer. The pool has two idle policies: workers can
park immediately, or briefly spin before parking.

## Construction

```magma
normal := try thread_pool.new(a, 4, 256)
spinning := try thread_pool.newSpinning(a, 4, 256, 4096)
```

`new(a, workerCount, queueCapacity)` creates the normal pool with the specified
initial queue capacity. An idle worker
registers itself as sleeping and waits using the platform generation-wait
backend. On Windows this uses `WaitOnAddress`; Unix platforms use the standard
library's mutex/condition fallback.

`newSpinning(a, workerCount, queueCapacity, spinCount)` creates the alternative
low-latency pool. When its queue becomes empty, each worker checks the atomic
work generation up to `spinCount` times. An LLVM `pause` instruction is issued
between checks. If submission changes the generation during that interval, the
worker checks the queue without entering the operating-system wait. If the
budget expires, the worker parks exactly like a normal pool worker.

`workerCount`, `queueCapacity`, and the spinning pool's `spinCount` must all be
greater than zero.

When the queue fills, submission doubles its capacity and linearizes queued
tasks into the new ring in FIFO order. Growth is amortized O(1) and occurs while
holding the queue mutex. Allocation failure or integer capacity exhaustion is
returned without modifying the existing queue.

`spinCount` is an iteration budget, not a duration. Its duration depends on the
processor, clock state, compiler target, and contention. Measure it on the
target hardware rather than treating a particular count as a time unit.

## Submission and lifetime

`submit(entry, context)` queues `entry(context)`. The context is borrowed and
must remain valid until that task completes. Submission grows a full queue
rather than blocking the submitting thread.

`wait()` blocks until all pending work is complete. `shutdown()` first drains
pending work, stops and joins all workers, and releases the pool. It consumes
the pool.

```magma
pool := try thread_pool.new(a, 4, 256)
try pool.submit(doWork, context)
try pool.wait()
try pool.shutdown()
```

## Spinner trade-offs

Spinning exchanges idle CPU time for lower dispatch latency. While a worker is
inside its spin phase, submission only advances the atomic work generation; it
does not need to wake a parked native thread. If a worker has already parked,
submission uses the normal wake path.

All workers in a spinning pool use the configured spin phase. Consequently,
large idle pools can briefly consume several logical CPUs and generate cache
coherency traffic. A single submission can also be noticed by several spinning
workers, which then compete for the queue lock. Prefer a small worker count or
a conservative spin budget unless the workload regularly has parallel bursts.

The included `samples/thread_pool_spinner_benchmark.mg` uses one worker and a
budget of 4,096 pauses. On the development machine, submissions arriving up to
50 microseconds after the previous task reached their callback in roughly
0.7-0.9 microseconds. At a 100-microsecond gap the spin budget had expired and
latency returned to roughly 22 microseconds, matching the normal parked pool.
These figures illustrate the cutoff and are not platform guarantees.

Use the normal pool for mostly idle or latency-insensitive work. Consider the
spinning pool when tasks arrive frequently enough that avoiding native wakeups
is worth the extra CPU usage.

Other relevant samples are `samples/thread_pool_creation_benchmark.mg` and
`samples/thread_pool_idle_benchmark.mg`.
