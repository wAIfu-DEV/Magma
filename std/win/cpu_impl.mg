mod cpu_impl_win
# Windows processor-count backend used by the portable cpu module.


use "std:c" c
use "std:cast" cast

# ALL_PROCESSOR_GROUPS. Counting all groups avoids the 64-processor limit of
# GetSystemInfo on large Windows machines.
ext ext_win32_GetActiveProcessorCount GetActiveProcessorCount(groupNumber c.unsigned_short) c.unsigned_int

pub coreCount() u64:
    ret cast.u32to64(ext_win32_GetActiveProcessorCount(0xFFFF))
..
