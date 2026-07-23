mod fs_impl_win
# Windows filesystem backend used by the portable fs module.


use "std:c" c
use "std:allocator" allocator
use "std:builder" builder
use "std:cast" cast
use "std:errors" errors
use "std:slices" slices
use "std:strings" strings
use "std:utf8" utf8

ext ext_FindFirstFileW FindFirstFileW(pattern c.unsigned_short*, data ptr) ptr
ext ext_FindNextFileW FindNextFileW(handle ptr, data ptr) c.int
ext ext_FindClose FindClose(handle ptr) c.int
ext ext_DeleteFileW DeleteFileW(path c.unsigned_short*) c.int
ext ext_GetLastError GetLastError() c.unsigned_int

pub removeFile(a allocator.Allocator, path str) !void:
    wide := try utf8.utf8To16NT(a, path)
    defer slices.free(a, wide)
    if ext_DeleteFileW(slices.toPtr(wide)) == 0:
        throw errors.native(ext_GetLastError(), "DeleteFileW failed")
    ..
..

join(a allocator.Allocator, left str, right str) !$str:
    out := try builder.new(a)
    defer out.free()
    try out.appendBorrowed(left)
    if strings.countBytes(left) > 0 && strings.byteAt(left, strings.countBytes(left) - 1) != 92 && strings.byteAt(left, strings.countBytes(left) - 1) != 47:
        try out.appendBorrowed("\\")
    ..
    try out.appendBorrowed(right)
    ret try out.build()
..

walkInner(a allocator.Allocator, root str, visit (str, bool) !void) !void:
    pattern := try join(a, root, "*")
    defer strings.free(a, pattern)
    widePattern := try utf8.utf8To16NT(a, pattern)
    defer slices.free(a, widePattern)

    # WIN32_FIND_DATAW is 592 bytes. Fixed arrays inside Magma structs are
    # slices, so use raw ABI storage and the documented field offsets.
    data := array u8[592]
    dataPtr := slices.toPtr(data)
    attributes u32* = dataPtr
    namePtr u16* = cast.utop(cast.ptou(dataPtr) + 44)
    handle := ext_FindFirstFileW(slices.toPtr(widePattern), dataPtr)
    if cast.ptou(handle) == cast.itou(-1):
        throw errors.native(ext_GetLastError(), "FindFirstFileW failed")
    ..
    defer ext_FindClose(handle)

    more bool = true
    while more:
        nameCount u64 = 0
        while nameCount < 260 && namePtr[nameCount] != 0:
            nameCount = nameCount + 1
        ..
        nameWide u16[] = slices.fromPtr(namePtr, nameCount)
        name := try utf8.utf16to8(a, nameWide)
        if strings.compare(name, ".") == false && strings.compare(name, "..") == false:
            child := try join(a, root, name)
            isDirectory bool = (*attributes & 0x10) != 0
            try visit(child, isDirectory)
            if isDirectory:
                try walkInner(a, child, visit)
            ..
            strings.free(a, child)
        ..
        strings.free(a, name)
        more = ext_FindNextFileW(handle, dataPtr) != 0
    ..
..

pub walk(a allocator.Allocator, root str, visit (str, bool) !void) !void:
    try walkInner(a, root, visit)
..
