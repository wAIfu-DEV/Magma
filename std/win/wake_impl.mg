mod wake_impl_win

use "../cast.mg" cast
use "../errors.mg" errors

const condition u8 = 0
const infinite u32 = 0xFFFFFFFF

Wake(
    strategy u8
    lock ptr
    conditionVariable ptr
    count u64
    semaphore ptr
)

ext ext_win32_CreateSemaphoreW CreateSemaphoreW(attributes ptr, initialCount i32, maximumCount i32, name ptr) ptr
ext ext_win32_ReleaseSemaphore ReleaseSemaphore(semaphore ptr, releaseCount i32, previousCount ptr) i32
ext ext_win32_WaitForSingleObject WaitForSingleObject(handle ptr, milliseconds u32) u32
ext ext_win32_CloseHandle CloseHandle(handle ptr) u32
ext ext_win32_GetLastError GetLastError() u32
ext ext_win32_AcquireSRWLockExclusive AcquireSRWLockExclusive(lock ptr) void
ext ext_win32_ReleaseSRWLockExclusive ReleaseSRWLockExclusive(lock ptr) void
ext ext_win32_SleepConditionVariableSRW SleepConditionVariableSRW(conditionVariable ptr, lock ptr, milliseconds u32, flags u32) i32
ext ext_win32_WakeConditionVariable WakeConditionVariable(conditionVariable ptr) void

pub new(strategy u8) !$Wake:
    value := Wake(strategy=strategy, lock=none, conditionVariable=none, count=0, semaphore=none)
    if strategy != condition:
        value.semaphore = ext_win32_CreateSemaphoreW(none, 0, 0x7FFFFFFF, none)
        if value.semaphore == none:
            throw errors.native(ext_win32_GetLastError(), "CreateSemaphoreW failed")
        ..
    ..
    ret value
..

pub wait(wake Wake*) !void:
    if wake.strategy == condition:
        ext_win32_AcquireSRWLockExclusive(addrof wake.lock)
        while wake.count == 0:
            ok i32 = ext_win32_SleepConditionVariableSRW(addrof wake.conditionVariable, addrof wake.lock, infinite, 0)
            if ok == 0:
                code u32 = ext_win32_GetLastError()
                ext_win32_ReleaseSRWLockExclusive(addrof wake.lock)
                throw errors.native(code, "SleepConditionVariableSRW failed")
            ..
        ..
        wake.count = wake.count - 1
        ext_win32_ReleaseSRWLockExclusive(addrof wake.lock)
        ret
    ..

    result u32 = ext_win32_WaitForSingleObject(wake.semaphore, infinite)
    if result != 0:
        throw errors.native(ext_win32_GetLastError(), "semaphore wait failed")
    ..
..

pub notify(wake Wake*) !void:
    if wake.strategy == condition:
        ext_win32_AcquireSRWLockExclusive(addrof wake.lock)
        wake.count = wake.count + 1
        ext_win32_WakeConditionVariable(addrof wake.conditionVariable)
        ext_win32_ReleaseSRWLockExclusive(addrof wake.lock)
        ret
    ..

    if ext_win32_ReleaseSemaphore(wake.semaphore, 1, none) == 0:
        throw errors.native(ext_win32_GetLastError(), "ReleaseSemaphore failed")
    ..
..

pub free(wake Wake*) !void:
    if wake.strategy != condition && wake.semaphore != none:
        if ext_win32_CloseHandle(wake.semaphore) == 0:
            throw errors.native(ext_win32_GetLastError(), "CloseHandle failed")
        ..
        wake.semaphore = none
    ..
..
