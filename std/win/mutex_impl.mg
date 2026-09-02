mod mutex_impl_win
# Windows three-state mutex. The uncontended path is entirely atomic; threads
# park through WaitOnAddress only after observing contention.

use "std:atomic" atomic
use "std:errors" errors
use "std:win/types" win

link "synchronization"

const unlocked u32 = 0
const locked u32 = 1
const contended u32 = 2
const infinite u32 = 0xFFFFFFFF

pub Mutex(
    state atomic.U32
)

ext ext_win32_WaitOnAddress WaitOnAddress(address win.PVOID, compareAddress win.PVOID, addressSize win.SIZE_T, milliseconds win.DWORD) win.BOOL
ext ext_win32_WakeByAddressSingle WakeByAddressSingle(address win.PVOID) void
ext ext_win32_GetLastError GetLastError() win.DWORD

compareExchange(value atomic.U32*, expected u32, desired u32) u32:
    unsafe:
        llvm "  %result = cmpxchg ptr %value, i32 %expected, i32 %desired acquire monotonic, align 4\n"
        llvm "  %observed = extractvalue { i32, i1 } %result, 0\n"
        llvm "  ret i32 %observed\n"
    ..
..

pub new() !Mutex:
    ret Mutex(state=atomic.newU32(unlocked))
..

pub lock(mutex Mutex*) !void:
    observed := compareExchange(addrof mutex.state, unlocked, locked)
    if observed == unlocked:
        ret
    ..
    if observed != contended:
        observed = mutex.state.exchange(contended)
    ..
    expected u32 = contended
    loop observed != unlocked:
        ok i32 = ext_win32_WaitOnAddress(addrof mutex.state, addrof expected, sizeof u32, infinite)
        if ok == 0:
            throw errors.native(ext_win32_GetLastError(), "WaitOnAddress failed")
        ..
        observed = mutex.state.exchange(contended)
    ..
..

pub unlock(mutex Mutex*) !void:
    previous := mutex.state.fetchSub(1)
    if previous == locked:
        ret
    ..
    mutex.state.storeRelease(unlocked)
    ext_win32_WakeByAddressSingle(addrof mutex.state)
..

pub free(mutex Mutex*) !void:
    mutex.state.store(unlocked)
..
