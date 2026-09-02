# `std/adaptive_spinlock`

`AdaptiveSpinLock` is a Windows and Unix lock for very short critical sections. Its
uncontended acquisition is one acquire compare-exchange and release is one
release store. After a failed acquisition it spins with `pause`, then yields to
the platform scheduler after 64 attempts.

`new()` creates an unlocked lock. `lock()` and `unlock()` are non-throwing, and
`locker()` returns a non-owning `std/locker.Locker` view.

Do not copy a lock after sharing it. Never hold it across blocking or
long-running work; use `std/mutex` when hold duration is unpredictable.
