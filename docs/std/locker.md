# `std/locker`

`Locker` is a type-erased mutual-exclusion `proto`. Implementations provide
`lockRaw`, `unlockRaw`, and `releaseRaw`; the public convenience methods are
`lock`, `unlock`, and the `free` destructor.

`lock() !void` acquires exclusive access, `unlock() !void` releases it, and the
destructor `free() void` invokes cleanup. Views returned by `Mutex.locker()` and
`SpinLock.locker()` are non-owning; the concrete lock must outlive the view.
