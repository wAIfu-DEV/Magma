mod cpu

@platform("windows")
use "win/cpu_impl.mg" impl_cpu

@platform("linux")
use "unix/cpu_impl_linux.mg" impl_cpu

@platform("android")
use "unix/cpu_impl_android.mg" impl_cpu

@platform("ios", "darwin", "freebsd", "openbsd")
use "unix/cpu_impl_posix.mg" impl_cpu

@platform("netbsd")
use "unix/cpu_impl_netbsd.mg" impl_cpu

# Returns the number of logical CPU cores currently available to the process.
# The result is always at least one, even when the operating-system query fails.
pub coreCount() u64:
    count u64 = impl_cpu.coreCount()
    if count == 0:
        ret 1
    ..
    ret count
..
