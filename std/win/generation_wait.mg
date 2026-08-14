mod generation_wait_win
# Windows generation-counter wait backend used by synchronization APIs.


use "std:win/types" win
use "std:errors" errors

link "synchronization"

const infinite u32 = 0xFFFFFFFF

pub Wait(
    marker u8
)

ext ext_win32_WaitOnAddress WaitOnAddress(address win.PVOID, compareAddress win.PVOID, addressSize win.SIZE_T, milliseconds win.DWORD) win.BOOL
ext ext_win32_WakeByAddressSingle WakeByAddressSingle(address win.PVOID) void
ext ext_win32_WakeByAddressAll WakeByAddressAll(address win.PVOID) void
ext ext_win32_GetLastError GetLastError() win.DWORD

pub new() !$Wait:
    ret Wait(marker=0)
..

pub observe(generation u32*) u32:
    # SAFETY: this audited implementation injects the required low-level IR.
    unsafe:
        llvm "  %value = load atomic i32, ptr %generation acquire, align 4\n"
            llvm "  ret i32 %value\n"
    ..
..

advance(generation u32*) void:
    # SAFETY: this audited implementation injects the required low-level IR.
    unsafe:
        llvm "  %previous = atomicrmw add ptr %generation, i32 1 release, align 4\n"
            llvm "  ret void\n"
    ..
..

pub signal(generation u32*) void:
    advance(generation)
..

pub wait(waiter Wait*, generation u32*, observed u32) !void:
    ok i32 = ext_win32_WaitOnAddress(generation, addrof observed, sizeof u32, infinite)
    if ok == 0:
        throw errors.native(ext_win32_GetLastError(), "WaitOnAddress failed")
    ..
..

pub wakeOne(waiter Wait*, generation u32*) void:
    advance(generation)
    ext_win32_WakeByAddressSingle(generation)
..

pub wakeAll(waiter Wait*, generation u32*, count u64) void:
    advance(generation)
    ext_win32_WakeByAddressAll(generation)
..

pub free(waiter Wait*) void:
..
