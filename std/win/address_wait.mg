mod address_wait_win
# Windows address-wait backend used by the portable wake module.


use "std:c" c
use "std:errors" errors

link "synchronization"

const infinite u32 = 0xFFFFFFFF

pub Wait(
    marker u8
)

ext ext_win32_WaitOnAddress WaitOnAddress(address ptr, compareAddress ptr, addressSize c.size_t, milliseconds c.unsigned_int) c.int
ext ext_win32_WakeByAddressSingle WakeByAddressSingle(address ptr) void
ext ext_win32_GetLastError GetLastError() c.unsigned_int

pub new() !$Wait:
    ret Wait(marker=0)
..

pub wait(waiter Wait*, status u32*) !void:
    pending u32 = 0
    ok i32 = ext_win32_WaitOnAddress(status, addrof pending, sizeof u32, infinite)
    if ok == 0:
        throw errors.native(ext_win32_GetLastError(), "WaitOnAddress failed")
    ..
..

pub wake(waiter Wait*, status u32*) void:
    ext_win32_WakeByAddressSingle(status)
..

pub free(waiter Wait*) void:
..
