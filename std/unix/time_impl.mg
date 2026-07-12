mod time_impl_unix

use "../cast.mg" cast

Timespec(
    sec i64,
    nsec i64,
)

ext ext_unix_clock_gettime clock_gettime(clockId i32, value Timespec*) i32

pub ticks() u64:
    value Timespec
    ext_unix_clock_gettime(1, addrof value)
    ret cast.itou((value.sec * 1000000000) + value.nsec)
..

pub tickFrequency() u64:
    ret 1000000000
..

pub unixTimestamp() u64:
    value Timespec
    ext_unix_clock_gettime(0, addrof value)
    ret cast.itou(value.sec)
..

pub unixTimestampMs() u64:
    value Timespec
    ext_unix_clock_gettime(0, addrof value)
    sec u64 = cast.itou(value.sec)
    nsec u64 = cast.itou(value.nsec)
    ret (sec * 1000) + (nsec / 1000000)
..

pub unixTimestampUs() u128:
    value Timespec
    ext_unix_clock_gettime(0, addrof value)
    sec u128 = cast.u64to128(cast.itou(value.sec))
    nsec u128 = cast.u64to128(cast.itou(value.nsec))
    ret (sec * 1000000) + (nsec / 1000)
..

pub unixTimestampNs() u128:
    value Timespec
    ext_unix_clock_gettime(0, addrof value)
    sec u128 = cast.u64to128(cast.itou(value.sec))
    nsec u128 = cast.u64to128(cast.itou(value.nsec))
    ret (sec * 1000000000) + nsec
..
