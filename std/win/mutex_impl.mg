mod mutex_impl_win

# SRWLOCK_INIT is a single zero-initialized pointer.
Mutex(
    state ptr
)

ext ext_win32_AcquireSRWLockExclusive AcquireSRWLockExclusive(mutex Mutex*) void
ext ext_win32_ReleaseSRWLockExclusive ReleaseSRWLockExclusive(mutex Mutex*) void

pub new() !Mutex:
    ret Mutex(state=none)
..

pub lock(mutex Mutex*) !void:
    ext_win32_AcquireSRWLockExclusive(mutex)
..

pub unlock(mutex Mutex*) !void:
    ext_win32_ReleaseSRWLockExclusive(mutex)
..

pub free(mutex Mutex*) !void:
    mutex.state = none
..
