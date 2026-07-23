mod mutex_impl_unix
# Unix mutex backend used by the portable mutex module.


use "std:c" c
@platform("linux", "freebsd", "netbsd", "openbsd")
link "pthread"

use "std:cast" cast
use "std:errors" errors

# Opaque, naturally aligned storage. 128 bytes covers the supported pthread ABIs.
pub Mutex(
    storage := array u64[16]
)

ext ext_pthread_mutex_init    pthread_mutex_init(mutex Mutex*, attributes ptr) c.int
ext ext_pthread_mutex_lock    pthread_mutex_lock(mutex Mutex*) c.int
ext ext_pthread_mutex_unlock  pthread_mutex_unlock(mutex Mutex*) c.int
ext ext_pthread_mutex_destroy pthread_mutex_destroy(mutex Mutex*) c.int

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
