# `std/async`

`async.Async` is a lightweight execution context for asynchronous
standard-library operations. It borrows a `thread_pool.ThreadPool` and an
allocator, avoiding repeated parameters while keeping the synchronous I/O
modules independent of the thread-pool stack.

```magma
as := async.new(pool, allocator)
pending := try as.read(source, 512)
bytes := try pending.await()
```

`Async.read` copies the `Reader` interface into its task, but the reader's
underlying implementation, pool, and allocator must remain valid until the
future completes. The returned string is owned by the context's allocator.

`Async` does not close or otherwise own its pool or allocator and is cheap to
copy.
