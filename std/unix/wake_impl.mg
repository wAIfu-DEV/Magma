mod wake_impl_unix
# Unix wait-and-notify backend used by the portable wake module.


use "std:c" c
@platform("linux", "freebsd", "netbsd", "openbsd")
link "pthread"

use "std:cast" cast
use "std:errors" errors

const condition u8 = 0

# Opaque, naturally aligned storage for pthread and POSIX semaphore objects.
pub Wake(
    strategy u8
    lock := array u64[16]
    conditionVariable := array u64[16]
    count u64
    semaphore := array u64[16]
)

ext ext_pthread_mutex_init pthread_mutex_init(mutex ptr, attributes ptr) c.int
ext ext_pthread_mutex_lock pthread_mutex_lock(mutex ptr) c.int
ext ext_pthread_mutex_unlock pthread_mutex_unlock(mutex ptr) c.int
ext ext_pthread_mutex_destroy pthread_mutex_destroy(mutex ptr) c.int
ext ext_pthread_cond_init pthread_cond_init(condition ptr, attributes ptr) c.int
ext ext_pthread_cond_wait pthread_cond_wait(condition ptr, mutex ptr) c.int
ext ext_pthread_cond_signal pthread_cond_signal(condition ptr) c.int
ext ext_pthread_cond_destroy pthread_cond_destroy(condition ptr) c.int
ext ext_sem_init sem_init(semaphore ptr, shared c.int, value c.unsigned_int) c.int
ext ext_sem_wait sem_wait(semaphore ptr) c.int
ext ext_sem_post sem_post(semaphore ptr) c.int
ext ext_sem_destroy sem_destroy(semaphore ptr) c.int

nativeError(code i32, message str) error:
    ret errors.native(cast.u64to32(cast.itou(cast.i32to64(code))), message)
..

pub new(strategy u8) !$Wake:
    value Wake
    value.strategy = strategy
    if strategy == condition:
        code i32 = ext_pthread_mutex_init(addrof value.lock, none)
        if code != 0:
            throw nativeError(code, "pthread_mutex_init failed")
        ..
        code = ext_pthread_cond_init(addrof value.conditionVariable, none)
        if code != 0:
            ext_pthread_mutex_destroy(addrof value.lock)
            throw nativeError(code, "pthread_cond_init failed")
        ..
    else:
        code = ext_sem_init(addrof value.semaphore, 0, 0)
        if code != 0:
            throw nativeError(code, "sem_init failed")
        ..
    ..
    ret value
..

pub wait(wake Wake*) !void:
    if wake.strategy == condition:
        code i32 = ext_pthread_mutex_lock(addrof wake.lock)
        if code != 0:
            throw nativeError(code, "pthread_mutex_lock failed")
        ..
        while wake.count == 0:
            code = ext_pthread_cond_wait(addrof wake.conditionVariable, addrof wake.lock)
            if code != 0:
                ext_pthread_mutex_unlock(addrof wake.lock)
                throw nativeError(code, "pthread_cond_wait failed")
            ..
        ..
        wake.count = wake.count - 1
        code = ext_pthread_mutex_unlock(addrof wake.lock)
        if code != 0:
            throw nativeError(code, "pthread_mutex_unlock failed")
        ..
        ret
    ..
    code = ext_sem_wait(addrof wake.semaphore)
    if code != 0:
        throw nativeError(code, "sem_wait failed")
    ..
..

pub notify(wake Wake*) !void:
    if wake.strategy == condition:
        code i32 = ext_pthread_mutex_lock(addrof wake.lock)
        if code != 0:
            throw nativeError(code, "pthread_mutex_lock failed")
        ..
        wake.count = wake.count + 1
        code = ext_pthread_cond_signal(addrof wake.conditionVariable)
        unlockCode i32 = ext_pthread_mutex_unlock(addrof wake.lock)
        if code != 0:
            throw nativeError(code, "pthread_cond_signal failed")
        ..
        if unlockCode != 0:
            throw nativeError(unlockCode, "pthread_mutex_unlock failed")
        ..
        ret
    ..
    code = ext_sem_post(addrof wake.semaphore)
    if code != 0:
        throw nativeError(code, "sem_post failed")
    ..
..

pub free(wake Wake*) !void:
    if wake.strategy == condition:
        code i32 = ext_pthread_cond_destroy(addrof wake.conditionVariable)
        if code != 0:
            throw nativeError(code, "pthread_cond_destroy failed")
        ..
        code = ext_pthread_mutex_destroy(addrof wake.lock)
        if code != 0:
            throw nativeError(code, "pthread_mutex_destroy failed")
        ..
    else:
        code = ext_sem_destroy(addrof wake.semaphore)
        if code != 0:
            throw nativeError(code, "sem_destroy failed")
        ..
    ..
..
