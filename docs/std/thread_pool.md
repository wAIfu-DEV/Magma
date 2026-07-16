# `std/thread_pool`

`ThreadPool` keeps a fixed number of native threads alive and sends borrowed
tasks to them through a bounded ring buffer. Workers block in the operating
system while no task is available.

Create a pool with `new(workerCount, queueCapacity)`. Workers use condition
variables internally; wake selection is deliberately not part of the public
pool API.

`submit(entry, context)` queues `entry(context)`. The context is borrowed and
must remain valid through `wait()` or `shutdown()`. A full bounded queue returns
a `wouldOverflow` error rather than blocking the submitting thread.

`wait()` blocks until all currently pending work is complete. Only one
concurrent waiter is currently supported. `shutdown()` waits for pending work,
wakes and joins all workers, and releases the pool. It consumes the pool.

```magma
pool := try thread_pool.new(4, 256)
try pool.submit(doWork, context)
try pool.wait()
try pool.shutdown()
```

`samples/thread_pool_benchmark.mg` compares task wake latency with native thread
creation. `samples/thread_pool_idle_benchmark.mg` compares idle process CPU time
against a sleeping baseline and includes a busy-loop control to validate the
CPU-time measurement.
