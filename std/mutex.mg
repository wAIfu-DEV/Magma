mod mutex

@platform("windows")
use "win/mutex_impl.mg" impl_mutex

@platform("linux", "android", "ios", "darwin", "freebsd", "netbsd", "openbsd")
use "unix/mutex_impl.mg" impl_mutex

Mutex(
    impl impl_mutex.Mutex
)

pub new() !$Mutex:
    impl impl_mutex.Mutex = try impl_mutex.new()
    ret Mutex(impl=impl)
..

Mutex.lock() !void:
    try impl_mutex.lock(addrof this.impl)
..

Mutex.unlock() !void:
    try impl_mutex.unlock(addrof this.impl)
..

destr Mutex.free() !void:
    try impl_mutex.free(addrof this.impl)
..
