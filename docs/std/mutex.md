# `std/mutex`

`Mutex` is an owned portable blocking mutex. `new() !$Mutex` creates it unlocked,
`lock() !void` waits for exclusive ownership, and `unlock() !void` releases it.

Do not copy a mutex after it is shared or locked. Call `free() !void` only while
it is unlocked and after all users have stopped. `locker()` returns a non-owning
`std/locker.Locker`; the mutex must outlive that view.

On Windows, `Mutex` uses an exclusive SRW lock. Its zero-initialized,
pointer-sized state requires no separate initialization or destruction call.
Locking and unlocking use `AcquireSRWLockExclusive` and
`ReleaseSRWLockExclusive`; Windows handles spinning and parking internally.
