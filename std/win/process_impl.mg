mod process_impl_win
# Windows child-process backend used by the portable process module.


use "std:win/types" win
use "std:allocator" allocator
use "std:heap" heap
use "std:strings" strings
use "std:slices" slices
use "std:utf8" utf8
use "std:errors" errors
use "std:cast" cast
use "std:memory" memory
use "std:checked" checked
use "std:utf16" utf16

ext ext_win32_CreateProcessW CreateProcessW(applicationName win.LPCWSTR, commandLineValue win.LPWSTR, processAttributes win.LPVOID, threadAttributes win.LPVOID, inheritHandles win.BOOL, creationFlags win.DWORD, environment win.LPVOID, currentDirectory win.LPCWSTR, startupInfo win.LPVOID, processInformation win.LPVOID) win.BOOL
ext ext_win32_WaitForSingleObject WaitForSingleObject(handle win.HANDLE, milliseconds win.DWORD) win.DWORD
ext ext_win32_GetExitCodeProcess GetExitCodeProcess(handle win.HANDLE, exitCode win.DWORD*) win.BOOL
ext ext_win32_TerminateProcess TerminateProcess(handle win.HANDLE, exitCode win.UINT) win.BOOL
ext ext_win32_CloseHandle CloseHandle(handle win.HANDLE) win.BOOL
ext ext_win32_GetLastError GetLastError() win.DWORD

StartupInfo(
    cb u32
    reserved ptr
    desktop ptr
    title ptr
    x u32
    y u32
    xSize u32
    ySize u32
    xCountChars u32
    yCountChars u32
    fillAttribute u32
    flags u32
    showWindow u16
    reserved2Bytes u16
    reserved2 u8*
    stdInput ptr
    stdOutput ptr
    stdError ptr
)

ProcessInformation(
    process ptr
    thread ptr
    processId u32
    threadId u32
)

pub Process(
    handle ptr
)

containsNull(value str) bool:
    i u64 = 0
    while i < value.countBytes():
        if strings.byteAt(value, i) == 0:
            ret true
        ..
        i = i + 1
    ..
    ret false
..

# Appends one argument using the CommandLineToArgvW quoting rules. The output
# allocation is deliberately sized for the worst case (every byte a slash).
appendQuoted(value str, out u8*, offset u64*) void:
    out[*offset] = 34
    *offset = *offset + 1
    slashes u64 = 0
    i u64 = 0
    while i < value.countBytes():
        byte u8 = strings.byteAt(value, i)
        if byte == 92:
            slashes = slashes + 1
        elif byte == 34:
            while slashes > 0:
                out[*offset] = 92
                *offset = *offset + 1
                out[*offset] = 92
                *offset = *offset + 1
                slashes = slashes - 1
            ..
            out[*offset] = 92
            *offset = *offset + 1
            out[*offset] = 34
            *offset = *offset + 1
        else:
            while slashes > 0:
                out[*offset] = 92
                *offset = *offset + 1
                slashes = slashes - 1
            ..
            out[*offset] = byte
            *offset = *offset + 1
        ..
        i = i + 1
    ..
    while slashes > 0:
        out[*offset] = 92
        *offset = *offset + 1
        out[*offset] = 92
        *offset = *offset + 1
        slashes = slashes - 1
    ..
    out[*offset] = 34
    *offset = *offset + 1
..

commandLine(a allocator.Allocator, executable str, arguments str[]) !$str:
    count u64 = slices.count(arguments)
    total u64 = executable.countBytes()
    i u64 = 0
    while i < count:
        n u64 = arguments[i].countBytes()
        if n > (0 - 1 - total):
            throw errors.wouldOverflow("process command line is too large")
        ..
        total = total + n
        i = i + 1
    ..
    if total > ((0 - 1 - ((count + 1) * 3)) / 2):
        throw errors.wouldOverflow("process command line is too large")
    ..
    capacity u64 = (total * 2) + ((count + 1) * 3)
    data u8* = try a.alloc(capacity + 1)
    offset u64 = 0
    appendQuoted(executable, data, addrof offset)
    i = 0
    while i < count:
        data[offset] = 32
        offset = offset + 1
        appendQuoted(arguments[i], data, addrof offset)
        i = i + 1
    ..
    data[offset] = 0
    ret strings.fromPtrNoCopy(data, offset)
..

pub spawn(executable str, arguments str[]) !$Process:
    if executable.countBytes() == 0 || containsNull(executable):
        throw errors.invalidArgument("process executable is empty or contains a null byte")
    ..
    i u64 = 0
    while i < slices.count(arguments):
        if containsNull(arguments[i]):
            throw errors.invalidArgument("process argument contains a null byte")
        ..
        i = i + 1
    ..

    a := heap.allocator()
    line str = try commandLine(a, executable, arguments)
    defer line.free(a)
    
    line16 u16[] = try utf8.utf8To16NT(a, line)
    defer a.free(slices.toPtr(line16))

    startup StartupInfo
    startup.cb = cast.u64to32(sizeof StartupInfo)
    information ProcessInformation
    # A null application name makes CreateProcess search the usual executable
    # locations while taking argv[0] from the quoted command line.
    ok win.BOOL = ext_win32_CreateProcessW(none, slices.toPtr(line16), none, none, 1, 0, none, none, addrof startup, addrof information)
    if ok == 0:
        throw errors.native(ext_win32_GetLastError(), "CreateProcessW failed")
    ..
    onerror ext_win32_CloseHandle(information.process)
    if ext_win32_CloseHandle(information.thread) == 0:
        code u32 = ext_win32_GetLastError()
        throw errors.native(code, "CloseHandle failed for process thread")
    ..
    ret Process(handle=information.process)
..

environmentBlock(a allocator.Allocator, entries str[]) !$u16*:
    count := slices.count(entries)
    total u64 = 1
    i u64 = 0
    while i < count:
        total = try checked.uAdd(total, try utf16.fromUtf8Size(entries[i]))
        total = try checked.uAdd(total, 1)
        i = i + 1
    ..
    block := try a.allocT[u16](total)
    offset u64 = 0
    onerror a.free(block)
    i = 0
    while i < count:
        encoded u16[] = try utf8.utf8To16(a, entries[i])
        units := slices.count(encoded)
        memory.copy(slices.toPtr(encoded), addrof block[offset], units * sizeof u16)
        a.free(slices.toPtr(encoded))
        offset = offset + units
        block[offset] = 0
        offset = offset + 1
        i = i + 1
    ..
    block[offset] = 0
    ret block
..

pub spawnWithEnv(executable str, arguments str[], environment str[]) !$Process:
    if executable.countBytes() == 0 || containsNull(executable):
        throw errors.invalidArgument("process executable is empty or contains a null byte")
    ..
    a := heap.allocator()
    line := try commandLine(a, executable, arguments)
    defer line.free(a)
    line16 := try utf8.utf8To16NT(a, line)
    defer a.free(slices.toPtr(line16))
    block := try environmentBlock(a, environment)
    defer a.free(block)
    startup StartupInfo
    startup.cb = cast.u64to32(sizeof StartupInfo)
    information ProcessInformation
    ok := ext_win32_CreateProcessW(none, slices.toPtr(line16), none, none, 1, 0x400, block, none, addrof startup, addrof information)
    if ok == 0: throw errors.native(ext_win32_GetLastError(), "CreateProcessW failed") ..
    ext_win32_CloseHandle(information.thread)
    ret Process(handle=information.process)
..

pub isFinished(process Process*) !bool:
    if process.handle == none:
        throw errors.invalidArgument("process has already been waited")
    ..
    result u32 = ext_win32_WaitForSingleObject(process.handle, 0)
    if result == 0:
        ret true
    elif result == 258:
        ret false
    elif result == 0xFFFFFFFF:
        throw errors.native(ext_win32_GetLastError(), "process status query failed")
    ..
    throw errors.failure("unexpected process wait result")
    ret false
..

pub await(process Process*) !u32:
    if process.handle == none:
        throw errors.invalidArgument("process has already been waited")
    ..
    result u32 = ext_win32_WaitForSingleObject(process.handle, 0xFFFFFFFF)
    if result != 0:
        if result == 0xFFFFFFFF:
            throw errors.native(ext_win32_GetLastError(), "process wait failed")
        ..
        throw errors.failure("unexpected process wait result")
    ..
    exitCode u32 = 0
    if ext_win32_GetExitCodeProcess(process.handle, addrof exitCode) == 0:
        throw errors.native(ext_win32_GetLastError(), "GetExitCodeProcess failed")
    ..
    if ext_win32_CloseHandle(process.handle) == 0:
        code u32 = ext_win32_GetLastError()
        process.handle = none
        throw errors.native(code, "CloseHandle failed for process")
    ..
    process.handle = none
    ret exitCode
..

pub kill(process Process*) !void:
    if process.handle == none:
        throw errors.invalidArgument("process has already been consumed")
    ..

    status u32 = ext_win32_WaitForSingleObject(process.handle, 0)
    if status == 258:
        if ext_win32_TerminateProcess(process.handle, 1) == 0:
            throw errors.native(ext_win32_GetLastError(), "TerminateProcess failed")
        ..
        status = ext_win32_WaitForSingleObject(process.handle, 0xFFFFFFFF)
    ..
    if status == 0xFFFFFFFF:
        throw errors.native(ext_win32_GetLastError(), "process wait after kill failed")
    elif status != 0:
        throw errors.failure("unexpected process wait result after kill")
    ..

    if ext_win32_CloseHandle(process.handle) == 0:
        code u32 = ext_win32_GetLastError()
        process.handle = none
        throw errors.native(code, "CloseHandle failed for process")
    ..
    process.handle = none
..
