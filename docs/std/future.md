# `std/future`

`Future[T]` represents a value being produced by a task submitted to a
`ThreadPool`. Future does not own a scheduler: the pool passed to `future.new`
selects the worker count, queue capacity, and idle policy.

```magma
pool := try thread_pool.new(a, 2, 8, 256, 0)
pending := try future.new[Data, LoadContext](a, pool, loadEntry, context)
value := try pending.await()
try pool.close()
```

`future.new` allocates one combined work/state object, initializes completion
state and reference ownership, creates the completion waiter, and submits a
generic task to the pool. The task publishes either its value or error and
wakes a consumer waiting in `await()`.

`isDone()` polls completion without consuming the Future. `await()` waits when
necessary, returns the produced value or error, and consumes the Future. A live
Future must ultimately be consumed according to Magma's destructor rules; it
is not implicitly detached.

## Effect of the ThreadPool idle policy

The Future API works with any pool spin budget without modification:

```magma
lowLatencyPool := try thread_pool.new(a, 1, 4, 8, 4096)
pending := try future.new[Data, LoadContext](a, lowLatencyPool, loadEntry, context)
data := try pending.await()
try lowLatencyPool.close()
```

With a normal pool, submitting a Future to an idle worker may require an
operating-system wake. This dominated the earlier parked-worker callback
benchmark: direct ThreadPool submission took about 43.1 microseconds and a
Future took about 44.3 microseconds. The roughly 1.2-microsecond difference was
Future allocation, initialization, completion setup, and generic submission
overhead; most of the total was parked-worker dispatch.

With a spinning pool, a Future submitted inside the spin window avoids that
native wake latency. It still pays all Future-specific costs, so its expected
startup is direct spinning-pool dispatch plus Future creation overhead. The
spinner does not make allocation, result publication, `await()`, or reference
release cheaper.

Once the spin budget expires, Future behavior and latency return to the normal
parked-worker path. Spinning therefore benefits frequent or bursty asynchronous
operations, but offers no startup advantage when operations are separated by
long idle periods. The CPU and contention costs described in
`docs/std/thread_pool.md` apply equally when the submitted tasks back Futures.

Pool construction time is outside Future creation and should normally be paid
once. Reuse a pool across asynchronous operations instead of creating a pool
per Future.

`samples/future_creation_benchmark.mg` measures creation-to-callback latency,
and `samples/future_stage_benchmark.mg` reports the phases inside `future.new`.
