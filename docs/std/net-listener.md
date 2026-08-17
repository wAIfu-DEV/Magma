# `std/net/listener`

Callback-driven TCP listener built from `std/net/tcp` and
`std/net/event_loop`. `AcceptCallback` has the shape
`(context ptr, stream $tcp.Stream) !void`; every callback receives ownership of
the accepted stream and must close it or transfer it onward.

- `new(a, endpoint, backlog, capacity, commandCapacity, callback, context)`
  creates a nonblocking server and registers its accept callback.
- `localEndpoint()` is useful after binding port zero.
- `run()` or `runOnce(timeoutMs)` dispatch accepts synchronously; `stop()`
  interrupts a running wait.
- `runAsync() !$RunningListener` consumes the listener and schedules its
  event loop. Call `RunningListener.stop()` and then consume it with `await()`;
  `await()` also closes the server and frees state.
- `Listener.close()` releases a listener that was not transferred to async
  execution.

The callback context is borrowed through the listener's complete lifetime.
Callback errors terminate the loop and are returned by `run`, `runOnce`, or
the eventual asynchronous `await`.
