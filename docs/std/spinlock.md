# `std/spinlock`

`SpinLock` is a portable busy-waiting lock for very short critical sections.
`new()` creates an unlocked lock. `lock()` spins and yields until it acquires
exclusive access; `unlock()` releases it. Both operations are non-throwing.

Do not copy a spin lock after sharing it or hold one across blocking or
long-running work; use `std/mutex` under contention. `locker()` returns a
non-owning `std/locker.Locker` view.
