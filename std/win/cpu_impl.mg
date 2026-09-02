mod cpu_impl_win
# Windows processor-count backend used by the portable cpu module.


use "std:win/types" win
use "std:cast" cast

# ALL_PROCESSOR_GROUPS. Counting all groups avoids the 64-processor limit of
# GetSystemInfo on large Windows machines.
ext ext_win32_GetActiveProcessorCount GetActiveProcessorCount(groupNumber win.WORD) win.DWORD

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
    count := cachedCount()
    if count != 0: ret count ..
    count = cast.u32to64(ext_win32_GetActiveProcessorCount(0xFFFF))
    if count != 0: publishCount(count) ..
    ret count
..
