mod wake_impl_win
# Windows wait-and-notify backend used by the portable wake module.


use "std:win/types" win
use "std:cast" cast
use "std:errors" errors

const condition u8 = 0
const infinite u32 = 0xFFFFFFFF

pub Wake(
    strategy u8
    lock ptr
    conditionVariable ptr
    count u64
    semaphore ptr
)

ext ext_win32_CreateSemaphoreW CreateSemaphoreW(attributes win.LPVOID, initialCount win.LONG, maximumCount win.LONG, name win.LPCWSTR) win.HANDLE
ext ext_win32_ReleaseSemaphore ReleaseSemaphore(semaphore win.HANDLE, releaseCount win.LONG, previousCount win.LONG*) win.BOOL
ext ext_win32_WaitForSingleObject WaitForSingleObject(handle win.HANDLE, milliseconds win.DWORD) win.DWORD
ext ext_win32_CloseHandle CloseHandle(handle win.HANDLE) win.BOOL
ext ext_win32_GetLastError GetLastError() win.DWORD
ext ext_win32_AcquireSRWLockExclusive AcquireSRWLockExclusive(lock win.PVOID) void
ext ext_win32_ReleaseSRWLockExclusive ReleaseSRWLockExclusive(lock win.PVOID) void
ext ext_win32_SleepConditionVariableSRW SleepConditionVariableSRW(conditionVariable win.PVOID, lock win.PVOID, milliseconds win.DWORD, flags win.ULONG) win.BOOL
ext ext_win32_WakeConditionVariable WakeConditionVariable(conditionVariable win.PVOID) void

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
