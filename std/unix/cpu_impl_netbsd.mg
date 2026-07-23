mod cpu_impl_netbsd
# NetBSD processor-count backend used by the portable cpu module.


use "std:c" c
use "std:cast" cast

ext ext_sysconf sysconf(name c.int) c.long

pub coreCount() u64:
    count i64 = ext_sysconf(1002)
    if count < 1:
        ret 0
    ..
    ret cast.itou(count)
..
