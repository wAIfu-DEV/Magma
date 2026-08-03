# `std/executor`

A type-erased task scheduler interface. `Executor` contains a borrowed
implementation pointer and immutable `Vtable` with `submit` and `free`
operations.

```magma
execution := pool.executor()
try execution.submit[Context](run, addrof context)
```

- `Executor.submit[Ctx](entry (Ctx*) u64, context Ctx*) !void` schedules
  `entry(context)`. The context remains caller-owned and must stay valid until
  the task finishes.
- `Executor.free() void` releases resources owned by the view. Borrowed
  adapters may implement it as a no-op; it does not release their scheduler.

`ThreadPool.executor()` returns an executor view of a thread pool. The pool
must remain alive until every submitted task has completed.
