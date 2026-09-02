mod env_impl_win
use "std:allocator" allocator
use "std:win/types" win
use "std:heap" heap
use "std:utf8" utf8
use "std:utf16" utf16
use "std:slices" slices
use "std:errors" errors
use "std:c" c
use "std:list" list

ext ext_GetEnvironmentVariableW GetEnvironmentVariableW(name win.LPCWSTR, value win.LPWSTR, size win.DWORD) win.DWORD
ext ext_SetEnvironmentVariableW SetEnvironmentVariableW(name win.LPCWSTR, value win.LPCWSTR) win.BOOL
ext ext_GetLastError GetLastError() win.DWORD
ext ext_GetEnvironmentStringsW GetEnvironmentStringsW() win.LPWSTR
ext ext_FreeEnvironmentStringsW FreeEnvironmentStringsW(block win.LPWSTR) win.BOOL

freeString(a allocator.Allocator, value $str) void:
    value.free(a)
..

name16(name str) !$u16[]:
    ret try utf8.utf8To16NT(name)
..

pub get(name str) !$str:
    temporary := ctx.alloc
    wide := try name16(name)
    defer heap.allocator().free(slices.toPtr(wide))
    local := array u16[256]
    written := ext_GetEnvironmentVariableW(slices.toPtr(wide), slices.toPtr(local), 256)
    if written == 0:
        code := ext_GetLastError()
        if code == 203:
            throw errors.notFound("environment variable was not found")
        ..
        if code != 0:
            throw errors.native(code, "GetEnvironmentVariableW failed")
        ..
        ret try utf16.toUtf8(ctx.alloc, slices.fromPtr(slices.toPtr(local), 0))
    ..
    if written < 256:
        ret try utf16.toUtf8(ctx.alloc, slices.fromPtr(slices.toPtr(local), written))
    ..
    buffer := try temporary.allocT[u16](written)
    needed := written
    written = ext_GetEnvironmentVariableW(slices.toPtr(wide), buffer, needed)
    if written == 0:
        code := ext_GetLastError()
        temporary.free(buffer)
        throw errors.native(code, "GetEnvironmentVariableW failed")
    elif written >= needed:
        temporary.free(buffer)
        throw errors.failure("environment variable changed while it was read")
    ..
    result str, conversionError error = utf16.toUtf8(ctx.alloc, slices.fromPtr(buffer, written))
    temporary.free(buffer)
    if conversionError.nok():
        throw conversionError
    ..
    ret result
..

pub has(name str) bool:
    wide u16[], e error = name16(name)
    if e.nok(): ret false ..
    defer heap.allocator().free(slices.toPtr(wide))
    needed := ext_GetEnvironmentVariableW(slices.toPtr(wide), none, 0)
    ret needed != 0 || ext_GetLastError() != 203
..

pub set(name str, value str) !void:
    n := try name16(name)
    defer heap.allocator().free(slices.toPtr(n))
    v := try utf8.utf8To16NT(value)
    defer heap.allocator().free(slices.toPtr(v))
    if ext_SetEnvironmentVariableW(slices.toPtr(n), slices.toPtr(v)) == 0:
        throw errors.native(ext_GetLastError(), "SetEnvironmentVariableW failed")
    ..
..

pub unset(name str) !void:
    n := try name16(name)
    defer heap.allocator().free(slices.toPtr(n))
    if ext_SetEnvironmentVariableW(slices.toPtr(n), none) == 0:
        throw errors.native(ext_GetLastError(), "SetEnvironmentVariableW failed")
    ..
..

pub list() !$list.List[str]:
    a := ctx.alloc
    block := ext_GetEnvironmentStringsW()
    if block == none: throw errors.native(ext_GetLastError(), "GetEnvironmentStringsW failed") ..
    entries := try list.new[str](a, freeString)
    onerror entries.free()
    offset u64 = 0
    loop block[offset] != 0:
        count u64 = 0
        loop block[offset + count] != 0: count = count + 1 ..
        value str, conversionError error = utf16.toUtf8(a, slices.fromPtr(addrof block[offset], count))
        if conversionError.nok():
            ext_FreeEnvironmentStringsW(block)
            throw conversionError
        ..
        try entries.pushRight(value)
        offset = offset + count + 1
    ..
    if ext_FreeEnvironmentStringsW(block) == 0:
        throw errors.native(ext_GetLastError(), "FreeEnvironmentStringsW failed")
    ..
    ret entries
..
