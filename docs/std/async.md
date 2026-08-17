# Asynchronous I/O

There is no `std:async` module. Asynchronous standard-library APIs compose
`std/context`, `std/executor`, and `std/future`; this page is a cross-module
guide retained at the former topic URL.

```magma
pool := try thread_pool.newDefault(a)
defer pool.close()
ctx = context.new(a, a, pool.executor())

source := try fileHandle.reader()
pending := try source.readAsync(512)
bytes := try pending.await()
```

Implicit `ctx` borrows allocators and an executor. `Reader.readAsync` copies the
`Reader` interface and requested byte count into private future work storage,
then performs `Reader.read` on the executor. The reader's implementation, the
executor, and the allocator must remain valid until `await()` consumes the
future. The returned string is owned by the context allocator.

Executable and native-thunk startup supplies a retained per-thread default context
when explicit scheduler shutdown is unnecessary. Use an explicit pool and
context when worker limits, allocation, sharing, or shutdown order matter.
