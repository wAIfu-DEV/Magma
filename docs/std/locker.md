# `std/locker`

`Locker` is a type-erased mutual-exclusion handle containing an implementation
pointer and a `Vtable` of `lock`, `unlock`, and `free` callbacks.

`lock() !void` acquires exclusive access, `unlock() !void` releases it, and the
destructor `free() void` invokes cleanup. Views returned by `Mutex.locker()` and
`SpinLock.locker()` are non-owning; the concrete lock must outlive the view.
