mod wake

use "errors.mg" errors

@platform("windows")
use "win/wake_impl.mg" impl_wake

@platform("linux", "android", "ios", "darwin", "freebsd", "netbsd", "openbsd")
use "unix/wake_impl.mg" impl_wake

# A condition variable plus an internal counter.
pub condition() u8:
    ret 0
..

# A native counting semaphore.
pub semaphore() u8:
    ret 1
..

Wake(
    impl impl_wake.Wake
)

pub new(strategy u8) !$Wake:
    if strategy != condition() && strategy != semaphore():
        throw errors.invalidArgument("unknown wake strategy")
    ..
    impl impl_wake.Wake = try impl_wake.new(strategy)
    ret Wake(impl=impl)
..

# Sleeps until a wake token is available, then consumes one token.
Wake.wait() !void:
    try impl_wake.wait(addrof this.impl)
..

# Adds one wake token and wakes one waiter.
Wake.notify() !void:
    try impl_wake.notify(addrof this.impl)
..

destr Wake.free() !void:
    try impl_wake.free(addrof this.impl)
..
