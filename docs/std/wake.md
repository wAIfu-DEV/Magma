# `std/wake`

`Wake` is an owned counted wait-and-notify primitive. Notifications are retained
as tokens, so `notify()` may happen before `wait()` without losing the signal.

Create one with `new(condition()) !$Wake` for a condition-variable-backed
implementation or `new(semaphore()) !$Wake` for a native counting semaphore.
`wait() !void` consumes a token, blocking if necessary; `notify() !void` adds a
token and wakes one waiter. Call `free() !void` only after all users have stopped.
