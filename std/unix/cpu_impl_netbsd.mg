mod cpu_impl_netbsd
# NetBSD processor-count backend used by the portable cpu module.


use "std:c" c
use "std:cast" cast

ext ext_sysconf sysconf(name c.int) c.long

llvm "@magma.cpu.count = internal global i64 0, align 8\n"

cachedCount() u64:
    unsafe:
        llvm "  %value = load atomic i64, ptr @magma.cpu.count acquire, align 8\n"
        llvm "  ret i64 %value\n"
    ..
..

publishCount(value u64) void:
    unsafe:
        llvm "  store atomic i64 %value, ptr @magma.cpu.count release, align 8\n"
        llvm "  ret void\n"
    ..
..

pub coreCount() u64:
    cached := cachedCount()
    if cached != 0: ret cached ..
    count i64 = ext_sysconf(1002)
    if count < 1:
        ret 0
    ..
    cached = cast.itou(count)
    publishCount(cached)
    ret cached
..
