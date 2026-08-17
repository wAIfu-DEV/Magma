# `std/future`

`Future[T]` represents a value being produced by a task submitted through an
`executor.Executor`. A future does not own that executor; a thread pool is one
implementation and controls worker count, queue capacity, and idle policy.

```magma
pool := try thread_pool.new(a, 2, 8, 256, 0)
scheduler := pool.executor()
pending := try future.new[Data, LoadContext](a, scheduler, loadEntry, context)
value := try pending.await()
try pool.close()
```

`new[T, Context](a, scheduler, entry (Context*) !T, context Context) !$Future[T]`
allocates one combined work/state object, initializes completion
state and reference ownership, creates the completion waiter, and submits a
generic task to the executor. The task publishes either its value or error and
wakes a consumer waiting in `await()`.

`isDone() !bool` polls completion without consuming the Future. The destructor
`await() !$T` waits when
necessary, returns the produced value or error, and consumes the Future. A live
Future must ultimately be consumed according to Magma's destructor rules; it
is not implicitly detached.

The Future API works with any pool spin budget. Pool construction is outside
Future creation, so reuse a pool across operations. The CPU and latency
trade-offs in [`thread_pool.md`](thread_pool.md) also apply to Future tasks.
