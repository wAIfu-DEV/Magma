mod cpu_impl_win

use "../cast.mg" cast

# ALL_PROCESSOR_GROUPS. Counting all groups avoids the 64-processor limit of
# GetSystemInfo on large Windows machines.
ext ext_win32_GetActiveProcessorCount GetActiveProcessorCount(groupNumber u16) u32

pub coreCount() u64:
    ret cast.u32to64(ext_win32_GetActiveProcessorCount(0xFFFF))
..
