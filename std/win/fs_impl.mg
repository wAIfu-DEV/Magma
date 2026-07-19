mod fs_impl_win

use "../allocator.mg" allocator
use "../builder.mg" builder
use "../cast.mg" cast
use "../errors.mg" errors
use "../slices.mg" slices
use "../strings.mg" strings
use "../utf8.mg" utf8

ext ext_FindFirstFileW FindFirstFileW(pattern u16*, data ptr) ptr
ext ext_FindNextFileW FindNextFileW(handle ptr, data ptr) i32
ext ext_FindClose FindClose(handle ptr) i32
ext ext_DeleteFileW DeleteFileW(path u16*) i32
ext ext_GetLastError GetLastError() u32

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
    data u8[592]
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
