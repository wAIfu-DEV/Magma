# `std/thread`

Portable native-thread creation for blocking work. Windows uses `CreateThread`;
Unix-family platforms use pthreads.

`Thread` is an owned, joinable native-thread handle. Create one with
`new[Ctx](entry (Ctx*) u64, context Ctx*) !$Thread`. The entry function executes as
`entry(context)` on the new thread. Return zero; the integer result is reserved
for the native platform adapter. The context is borrowed: it and everything
reachable through it must remain valid until the entry function finishes.

A thread is consumed by `Thread.join() !void`, which waits for completion and
releases its native resources. Detached threads are deliberately not exposed:
safe detached state reclamation will require the planned atomic or
reference-counted synchronization layer.

`Thread.isFinished() !bool` performs a non-blocking status check. A `true`
result means the entry function has returned, but the thread remains joinable
and still must be joined. This supports polling loops that perform other work
between checks.

`joinAll(threads Thread[]) !void` joins every element in a slice. It attempts
all joins even when one fails, then returns the first error. Every element is
consumed and must not be used afterwards.

`yield()` asks the operating system to schedule another ready thread.

`join()` establishes the synchronization point needed to read memory written by
the worker. Concurrent access before joining still requires synchronization;
this module does not yet provide mutexes or atomics.

```magma
use "../std/thread.mg" thread

worker(context ptr) u64:
    result u64* = context
    result[0] = 42
    ret 0
..

result u64 = 0
t := try thread.new[ptr](worker, addrof result)
try t.join()
```
