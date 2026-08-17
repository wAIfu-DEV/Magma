# `std/net/poll`

Portable, allocation-free readiness polling with cross-thread interruption.

- Flags are `READ`, `WRITE`, `ERROR`, and `HANGUP`; combine them with `|`.
- `Event(token, flags)` reports the caller-provided token and ready flags.
- `new(a, capacity) !$Poller` allocates a poller with nonzero capacity.
- `add(socket, token, flags)`, `modify(...)`, and `remove(socket)` manage
  borrowed socket pointers. Registered sockets must remain alive and stable.
- `wait(output, timeoutMs)` writes at most `min(output.count(), capacity)`
  events. A negative timeout waits indefinitely and zero does not block.
- `interrupt()` wakes a blocked wait and is safe from another thread.
- `close()` releases the backend.

An interrupt is a wake-up mechanism, not a synthetic socket event.
