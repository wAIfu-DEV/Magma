mod mutex_impl_unix

@platform("linux", "freebsd", "netbsd", "openbsd")
link "pthread"

use "../cast.mg" cast
use "../errors.mg" errors

# Opaque, naturally aligned storage. 128 bytes covers the supported pthread ABIs.
Mutex(
    storage u64[16]
)

ext ext_pthread_mutex_init    pthread_mutex_init(mutex Mutex*, attributes ptr) i32
ext ext_pthread_mutex_lock    pthread_mutex_lock(mutex Mutex*) i32
ext ext_pthread_mutex_unlock  pthread_mutex_unlock(mutex Mutex*) i32
ext ext_pthread_mutex_destroy pthread_mutex_destroy(mutex Mutex*) i32

nativeError(code i32, message str) error:
    ret errors.native(cast.u64to32(cast.itou(cast.i32to64(code))), message)
..

pub new() !Mutex:
    value Mutex
    code i32 = ext_pthread_mutex_init(addrof value, none)
    if code != 0:
        throw nativeError(code, "pthread_mutex_init failed")
    ..
    ret value
..

pub lock(mutex Mutex*) !void:
    code i32 = ext_pthread_mutex_lock(mutex)
    if code != 0:
        throw nativeError(code, "pthread_mutex_lock failed")
    ..
..

pub unlock(mutex Mutex*) !void:
    code i32 = ext_pthread_mutex_unlock(mutex)
    if code != 0:
        throw nativeError(code, "pthread_mutex_unlock failed")
    ..
..

pub free(mutex Mutex*) !void:
    code i32 = ext_pthread_mutex_destroy(mutex)
    if code != 0:
        throw nativeError(code, "pthread_mutex_destroy failed")
    ..
..
