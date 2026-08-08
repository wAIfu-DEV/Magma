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
    ret try utf8.utf8To16NT(heap.allocator(), name)
..

pub get(a allocator.Allocator, name str) !$str:
    wide := try name16(name)
    defer heap.allocator().free(slices.toPtr(wide))
    needed := ext_GetEnvironmentVariableW(slices.toPtr(wide), none, 0)
    if needed == 0:
        if ext_GetLastError() == 203:
            throw errors.notFound("environment variable was not found")
        ..
        throw errors.native(ext_GetLastError(), "GetEnvironmentVariableW failed")
    ..
    buffer := try a.allocT[u16](needed)
    written := ext_GetEnvironmentVariableW(slices.toPtr(wide), buffer, needed)
    if written == 0 && ext_GetLastError() != 0:
        a.free(buffer)
        throw errors.native(ext_GetLastError(), "GetEnvironmentVariableW failed")
    ..
    result str, conversionError error = utf16.toUtf8(a, slices.fromPtr(buffer, written))
    a.free(buffer)
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
    v := try utf8.utf8To16NT(heap.allocator(), value)
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

pub list(a allocator.Allocator) !$list.List[str]:
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
