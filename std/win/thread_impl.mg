mod thread_impl_win

use "../cast.mg" cast
use "../errors.mg" errors

ext ext_win32_CreateThread       CreateThread(attributes ptr, stackSize u64, startAddress (ptr) u64, parameter ptr, creationFlags u32, threadId ptr) ptr
ext ext_win32_WaitForSingleObject WaitForSingleObject(handle ptr, milliseconds u32) u32
ext ext_win32_CloseHandle        CloseHandle(handle ptr) u32
ext ext_win32_GetLastError       GetLastError() u32
ext ext_win32_SwitchToThread     SwitchToThread() u32

Thread(
    handle ptr
)

pub spawn(entry (ptr) u64, context ptr) !$Thread:
    if entry == none:
        throw errors.invalidArgument("thread entry is null")
    ..

    handle ptr = ext_win32_CreateThread(none, 0, entry, context, 0, none)
    if handle == none:
        code u32 = ext_win32_GetLastError()
        throw errors.native(code, "CreateThread failed")
    ..
    ret Thread(handle=handle)
..

pub isFinished(thread Thread*) !bool:
    if thread.handle == none:
        throw errors.invalidArgument("thread is not joinable")
    ..
    result u32 = ext_win32_WaitForSingleObject(thread.handle, 0)
    if result == 0:
        ret true
    elif result == 258:
        ret false
    elif result == 0xFFFFFFFF:
        throw errors.native(ext_win32_GetLastError(), "thread status query failed")
    ..
    throw errors.failure("unexpected thread status result")
    ret false
..

pub join(thread Thread*) !bool:
    if thread.handle == none:
        throw errors.invalidArgument("thread is not joinable")
    ..

    result u32 = ext_win32_WaitForSingleObject(thread.handle, 0xFFFFFFFF)
    if result == 0xFFFFFFFF:
        waitCode u32 = ext_win32_GetLastError()
        ext_win32_CloseHandle(thread.handle)
        thread.handle = none
        throw errors.native(waitCode, "WaitForSingleObject failed")
    ..
    if result != 0:
        ext_win32_CloseHandle(thread.handle)
        thread.handle = none
        throw errors.failure("unexpected thread wait result")
    ..

    if ext_win32_CloseHandle(thread.handle) == 0:
        closeCode u32 = ext_win32_GetLastError()
        thread.handle = none
        throw errors.native(closeCode, "CloseHandle failed")
    ..
    thread.handle = none
    ret true
..

pub yield() void:
    ext_win32_SwitchToThread()
..
