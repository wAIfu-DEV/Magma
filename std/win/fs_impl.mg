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
ext ext_CreateDirectoryW CreateDirectoryW(path c.unsigned_short*, security ptr) c.int
ext ext_RemoveDirectoryW RemoveDirectoryW(path c.unsigned_short*) c.int
ext ext_MoveFileExW MoveFileExW(source c.unsigned_short*, destination c.unsigned_short*, flags c.unsigned_int) c.int
ext ext_CopyFileW CopyFileW(source c.unsigned_short*, destination c.unsigned_short*, failIfExists c.int) c.int
ext ext_GetCurrentDirectoryW GetCurrentDirectoryW(size c.unsigned_int, buffer c.unsigned_short*) c.unsigned_int
ext ext_SetCurrentDirectoryW SetCurrentDirectoryW(path c.unsigned_short*) c.int
ext ext_GetTempPathW GetTempPathW(size c.unsigned_int, buffer c.unsigned_short*) c.unsigned_int
ext ext_GetFullPathNameW GetFullPathNameW(path c.unsigned_short*, size c.unsigned_int, buffer c.unsigned_short*, filePart ptr) c.unsigned_int
ext ext_GetFileAttributesW GetFileAttributesW(path c.unsigned_short*) c.unsigned_int
ext ext_SetFileAttributesW SetFileAttributesW(path c.unsigned_short*, attributes c.unsigned_int) c.int
ext ext_CreateFileW CreateFileW(path c.unsigned_short*, access c.unsigned_int, share c.unsigned_int, security ptr, disposition c.unsigned_int, attributes c.unsigned_int, template ptr) ptr
ext ext_GetFinalPathNameByHandleW GetFinalPathNameByHandleW(handle ptr, buffer c.unsigned_short*, size c.unsigned_int, flags c.unsigned_int) c.unsigned_int
ext ext_CloseHandle CloseHandle(handle ptr) c.int

pub NativeMetadata(
    kind u8
    size u64
    permissions u32
    modified i64
)

pub Entry(
    name str
    kind u8
)

pub Dir(
    allocator allocator.Allocator
    handle ptr
    data ptr
    pending bool
    open bool
    currentName str
    hasCurrent bool
)

Dir.clearCurrent() void:
    if this.hasCurrent:
        this.currentName.free(this.allocator)
        this.hasCurrent = false
    ..
..

Dir.readEntry() !$Entry:
    this.clearCurrent()
    attributes u32* = this.data
    namePtr u16* = cast.utop(cast.ptou(this.data) + 44)
    count u64 = 0
    while count < 260 && namePtr[count] != 0:
        count = count + 1
    ..
    units u16[] = slices.fromPtr(namePtr, count)
    this.currentName = try utf8.utf16to8(this.allocator, units)
    this.hasCurrent = true
    kind u8 = 1
    if (*attributes & 0x400) != 0:
        kind = 3
    elif (*attributes & 0x10) != 0:
        kind = 2
    ..
    ret Entry(name=this.currentName, kind=kind)
..

pub Dir.hasData() bool:
    ret this.open && this.pending
..

pub Dir.next() !$Entry:
    while this.pending:
        result := try this.readEntry()
        if ext_FindNextFileW(this.handle, this.data) == 0:
            code := ext_GetLastError()
            this.pending = false
            if code != 18:
                this.clearCurrent()
                throw errors.native(code, "FindNextFileW failed")
            ..
        ..
        if strings.compare(result.name, ".") == false && strings.compare(result.name, "..") == false:
            ret result
        ..
        this.clearCurrent()
    ..
    throw errors.notFound("directory iterator exhausted")
..

destr Dir.close() !void:
    this.clearCurrent()
    if this.open:
        if ext_FindClose(this.handle) == 0:
            throw errors.native(ext_GetLastError(), "FindClose failed")
        ..
        this.allocator.free(this.data)
        this.open = false
        this.pending = false
    ..
..

pub openDir(a allocator.Allocator, path str) !$Dir:
    pattern := try join(a, path, "*")
    defer pattern.free(a)
    wide := try utf8.utf8To16NT(a, pattern)
    defer slices.free(a, wide)
    data := try a.alloc(592)
    handle := ext_FindFirstFileW(slices.toPtr(wide), data)
    if cast.ptou(handle) == cast.itou(-1):
        code := ext_GetLastError()
        a.free(data)
        # FindFirst reports an empty directory as file-not-found.
        if code == 2:
            ret Dir(allocator=a, handle=none, data=none, pending=false, open=false, currentName="", hasCurrent=false)
        ..
        throw errors.native(code, "FindFirstFileW failed")
    ..
    ret Dir(allocator=a, handle=handle, data=data, pending=true, open=true, currentName="", hasCurrent=false)
..

metadataFromData(data ptr) NativeMetadata:
    attributes u32* = data
    kind u8 = 1
    if (*attributes & 0x400) != 0:
        kind = 3
    elif (*attributes & 0x10) != 0:
        kind = 2
    ..
    high u32* = cast.utop(cast.ptou(data) + 28)
    low u32* = cast.utop(cast.ptou(data) + 32)
    size := (cast.u32to64(*high) << 32) | cast.u32to64(*low)
    modifiedRaw u64* = cast.utop(cast.ptou(data) + 20)
    modified i64 = 0
    if *modifiedRaw >= 116444736000000000:
        modified = cast.utoi((*modifiedRaw - 116444736000000000) / 10000000)
    ..
    permissions u32 = 1
    if (*attributes & 1) == 0:
        permissions = permissions | 2
    ..
    ret NativeMetadata(kind=kind, size=size, permissions=permissions, modified=modified)
..

pub metadata(a allocator.Allocator, path str, followLinks bool) !NativeMetadata:
    if followLinks:
        resolved := try canonicalize(a, path)
        defer resolved.free(a)
        ret try metadata(a, resolved, false)
    ..
    wide := try utf8.utf8To16NT(a, path)
    defer slices.free(a, wide)
    data := array u8[592]
    handle := ext_FindFirstFileW(slices.toPtr(wide), slices.toPtr(data))
    if cast.ptou(handle) == cast.itou(-1):
        throw errors.native(ext_GetLastError(), "FindFirstFileW failed")
    ..
    defer ext_FindClose(handle)
    ret metadataFromData(slices.toPtr(data))
..

pub setPermissions(a allocator.Allocator, path str, permissions u32) !void:
    wide := try utf8.utf8To16NT(a, path)
    defer slices.free(a, wide)
    attributes := ext_GetFileAttributesW(slices.toPtr(wide))
    if attributes == 0xFFFFFFFF:
        throw errors.native(ext_GetLastError(), "GetFileAttributesW failed")
    ..
    if (permissions & 2) != 0:
        attributes = attributes & 0xFFFFFFFE
    else:
        attributes = attributes | 1
    ..
    if ext_SetFileAttributesW(slices.toPtr(wide), attributes) == 0:
        throw errors.native(ext_GetLastError(), "SetFileAttributesW failed")
    ..
..

pathCall(a allocator.Allocator, path str, operation (c.unsigned_short*) c.int, message str) !void:
    wide := try utf8.utf8To16NT(a, path)
    defer slices.free(a, wide)
    if operation(slices.toPtr(wide)) == 0:
        throw errors.native(ext_GetLastError(), message)
    ..
..

pub makeDir(a allocator.Allocator, path str) !void:
    wide := try utf8.utf8To16NT(a, path)
    defer slices.free(a, wide)
    if ext_CreateDirectoryW(slices.toPtr(wide), none) == 0:
        throw errors.native(ext_GetLastError(), "CreateDirectoryW failed")
    ..
..

pub removeDir(a allocator.Allocator, path str) !void:
    try pathCall(a, path, ext_RemoveDirectoryW, "RemoveDirectoryW failed")
..

pub setCurrentDir(a allocator.Allocator, path str) !void:
    try pathCall(a, path, ext_SetCurrentDirectoryW, "SetCurrentDirectoryW failed")
..

move(a allocator.Allocator, source str, destination str, flags u32) !void:
    sourceWide := try utf8.utf8To16NT(a, source)
    defer slices.free(a, sourceWide)
    destinationWide := try utf8.utf8To16NT(a, destination)
    defer slices.free(a, destinationWide)
    if ext_MoveFileExW(slices.toPtr(sourceWide), slices.toPtr(destinationWide), flags) == 0:
        throw errors.native(ext_GetLastError(), "MoveFileExW failed")
    ..
..

pub rename(a allocator.Allocator, source str, destination str) !void:
    try move(a, source, destination, 0)
..

pub replace(a allocator.Allocator, source str, destination str) !void:
    try move(a, source, destination, 9)
..

pub copyFile(a allocator.Allocator, source str, destination str) !void:
    sourceWide := try utf8.utf8To16NT(a, source)
    defer slices.free(a, sourceWide)
    destinationWide := try utf8.utf8To16NT(a, destination)
    defer slices.free(a, destinationWide)
    if ext_CopyFileW(slices.toPtr(sourceWide), slices.toPtr(destinationWide), 0) == 0:
        throw errors.native(ext_GetLastError(), "CopyFileW failed")
    ..
..

wideResult(a allocator.Allocator, temp bool) !$str:
    needed u32 = 0
    if temp:
        needed = ext_GetTempPathW(0, none)
    else:
        needed = ext_GetCurrentDirectoryW(0, none)
    ..
    if needed == 0:
        throw errors.native(ext_GetLastError(), "path query failed")
    ..
    buffer := try a.allocT[u16](cast.u32to64(needed) + 1)
    defer a.free(buffer)
    written u32 = 0
    if temp:
        written = ext_GetTempPathW(needed + 1, buffer)
    else:
        written = ext_GetCurrentDirectoryW(needed + 1, buffer)
    ..
    if written == 0:
        throw errors.native(ext_GetLastError(), "path query failed")
    ..
    ret try utf8.utf16to8(a, slices.fromPtr(buffer, cast.u32to64(written)))
..

pub currentDir(a allocator.Allocator) !$str:
    ret try wideResult(a, false)
..

pub temporaryDir(a allocator.Allocator) !$str:
    ret try wideResult(a, true)
..

pub canonicalize(a allocator.Allocator, path str) !$str:
    wide := try utf8.utf8To16NT(a, path)
    defer slices.free(a, wide)
    handle := ext_CreateFileW(slices.toPtr(wide), 0, 7, none, 3, 0x02000000, none)
    if cast.ptou(handle) == cast.itou(-1):
        throw errors.native(ext_GetLastError(), "CreateFileW failed")
    ..
    defer ext_CloseHandle(handle)
    needed := ext_GetFinalPathNameByHandleW(handle, none, 0, 0)
    if needed == 0:
        throw errors.native(ext_GetLastError(), "GetFinalPathNameByHandleW failed")
    ..
    buffer := try a.allocT[u16](cast.u32to64(needed) + 1)
    defer a.free(buffer)
    written := ext_GetFinalPathNameByHandleW(handle, buffer, needed + 1, 0)
    if written == 0:
        throw errors.native(ext_GetLastError(), "GetFinalPathNameByHandleW failed")
    ..
    ret try utf8.utf16to8(a, slices.fromPtr(buffer, cast.u32to64(written)))
..

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
    if left.countBytes() > 0 && strings.byteAt(left, left.countBytes() - 1) != 92 && strings.byteAt(left, left.countBytes() - 1) != 47:
        try out.appendBorrowed("\\")
    ..
    try out.appendBorrowed(right)
    ret try out.build()
..

walkInner(a allocator.Allocator, root str, visit (str, bool) !void) !void:
    pattern := try join(a, root, "*")
    defer pattern.free(a)
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
            child.free(a)
        ..
        name.free(a)
        more = ext_FindNextFileW(handle, dataPtr) != 0
    ..
..

pub walk(a allocator.Allocator, root str, visit (str, bool) !void) !void:
    try walkInner(a, root, visit)
..
