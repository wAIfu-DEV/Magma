mod mutex_impl_win
# Windows mutex backend used by the portable mutex module.


use "std:c" c
# SRWLOCK_INIT is a single zero-initialized pointer.
pub Mutex(
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
