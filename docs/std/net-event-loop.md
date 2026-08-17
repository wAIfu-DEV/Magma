# `std/net/event_loop`

Readiness callback loop that can run synchronously or on an existing executor.
`Callback` has the shape `(context ptr, token u64, flags u32) !void`.

`new(a, capacity, commandCapacity)` creates a loop with fixed registration and
cross-thread command capacities. Both must be nonzero.

- `watch(socket, token, flags, callback, context)` registers a unique token.
  The socket and callback context are borrowed and must remain valid until
  unregistered or until the loop is stopped and awaited.
- `modify(token, flags)` and `unwatch(token)` update registrations.
- `runOnce(timeoutMs)` performs one wait/dispatch cycle; `run()` blocks until
  `stop()` is called. Callback errors stop the run and propagate.
- `runAsync() !$RunningLoop` consumes the `EventLoop`, schedules it through
  `ctx.exec`, and transfers its state to a `RunningLoop`.
- `RunningLoop.watch`, `modify`, and `unwatch` enqueue cross-thread commands.
  They can fail when `commandCapacity` is full. `stop()` requests shutdown;
  `await()` must then consume the running loop and release its resources.
- `EventLoop.close()` releases a loop that was not transferred to async work.

Do not call the synchronous `EventLoop.watch` while its loop is running
asynchronously. Tokens are application identifiers; readiness flags come from
[`std/net/poll`](net-poll.md).
