mod mutex
# Portable blocking mutual exclusion implementing the Locker interface.

use "std:locker" locker

@platform("windows")
use "std:win/mutex_impl" impl_mutex

@platform("linux", "android", "ios", "darwin", "freebsd", "netbsd", "openbsd")
use "std:unix/mutex_impl" impl_mutex

# Owned native mutex providing blocking mutual exclusion between threads.
# @warning Do not copy a Mutex after it has been shared or locked.
pub Mutex(
    impl impl_mutex.Mutex
)

# Creates an unlocked mutex.
# @complexity O(1), excluding platform initialization cost
# @ownership Release with Mutex.free after all users have stopped.
# @example
#   guard := try mutex.new()
pub new() !$Mutex:
    impl impl_mutex.Mutex = try impl_mutex.new()
    ret Mutex(impl=impl)
..

# Blocks until the current thread acquires the mutex.
# @complexity O(1) when uncontended; blocking under contention
# @example
#   try guard.lock()
Mutex.lock() !void:
    try impl_mutex.lock(addrof this.impl)
..

# Releases a mutex held by the current thread.
# @complexity O(1)
# @warning The current thread must hold the mutex exactly once.
Mutex.unlock() !void:
    try impl_mutex.unlock(addrof this.impl)
..

# Releases native mutex resources.
# @complexity O(1)
# @warning The mutex must be unlocked and have no current or future waiters.
destr Mutex.free() !void:
    try impl_mutex.free(addrof this.impl)
..

lockerLock(raw ptr) !void:
    impl Mutex* = raw
    try impl.lock()
..

lockerUnlock(raw ptr) !void:
    impl Mutex* = raw
    try impl.unlock()
..

lockerFree(raw ptr) void:
    ret
..

const vtable := locker.Vtable(
    lock = lockerLock
    unlock = lockerUnlock
    free = lockerFree
)

# Returns a non-owning type-erased view of this mutex.
# @complexity O(1)
# @ownership The Mutex must outlive the returned Locker.
# @example
#   lock := guard.locker()
Mutex.locker() locker.Locker:
    ret locker.Locker(
        impl = this,
        vtable = addrof vtable,
    )
..
