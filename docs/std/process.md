# `std/process`

Portable child-process creation, waiting, termination, and asynchronous
execution. Children inherit the parent's environment and standard streams.

`spawn(executable, arguments) !$Process` starts a child; the executable becomes
`argv[0]`, so do not repeat it in `arguments`. A process must be consumed exactly
once by `await() !u32` or `kill() !void`. `isFinished() !bool` only polls: even
after it returns true, `await` is required. On Unix, signal termination is
reported as 128 plus the signal number.

`exec` combines spawn and await. `execAsync(executable, arguments)` returns
`future.Future[u32]` and uses the allocator and executor in implicit `ctx`; its
strings and slice are borrowed and must remain valid until the future is
awaited.

`spawnWithEnv(executable, arguments, environment)` replaces the child's entire
environment with `name=value` entries. It does not merge with or mutate the
parent environment. On Unix this calls `execve`, so `executable` must be an
explicit path; ordinary `spawn` retains PATH lookup through `execvp`.
