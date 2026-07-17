mod cpu_impl_netbsd

use "../cast.mg" cast

ext ext_sysconf sysconf(name i32) i64

pub coreCount() u64:
    count i64 = ext_sysconf(1002)
    if count < 1:
        ret 0
    ..
    ret cast.itou(count)
..
