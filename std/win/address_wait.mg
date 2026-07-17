mod address_wait_win

use "../errors.mg" errors

link "synchronization"

const infinite u32 = 0xFFFFFFFF

Wait(
    marker u8
)

ext ext_win32_WaitOnAddress WaitOnAddress(address ptr, compareAddress ptr, addressSize u64, milliseconds u32) i32
ext ext_win32_WakeByAddressSingle WakeByAddressSingle(address ptr) void
ext ext_win32_GetLastError GetLastError() u32

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
