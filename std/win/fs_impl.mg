mod fs_impl_win
# Windows filesystem backend used by the portable fs module.


use "std:win/types" win
use "std:allocator" allocator
use "std:builder" builder
use "std:cast" cast
use "std:errors" errors
use "std:slices" slices
use "std:strings" strings
use "std:utf8" utf8

ext ext_FindFirstFileW FindFirstFileW(pattern win.LPCWSTR, data win.LPVOID) win.HANDLE
ext ext_FindNextFileW FindNextFileW(handle win.HANDLE, data win.LPVOID) win.BOOL
ext ext_FindClose FindClose(handle win.HANDLE) win.BOOL
ext ext_DeleteFileW DeleteFileW(path win.LPCWSTR) win.BOOL
ext ext_GetLastError GetLastError() win.DWORD
ext ext_CreateDirectoryW CreateDirectoryW(path win.LPCWSTR, security win.LPVOID) win.BOOL
ext ext_RemoveDirectoryW RemoveDirectoryW(path win.LPCWSTR) win.BOOL
ext ext_MoveFileExW MoveFileExW(source win.LPCWSTR, destination win.LPCWSTR, flags win.DWORD) win.BOOL
ext ext_CopyFileW CopyFileW(source win.LPCWSTR, destination win.LPCWSTR, failIfExists win.BOOL) win.BOOL
ext ext_GetCurrentDirectoryW GetCurrentDirectoryW(size win.DWORD, buffer win.LPWSTR) win.DWORD
ext ext_SetCurrentDirectoryW SetCurrentDirectoryW(path win.LPCWSTR) win.BOOL
ext ext_GetTempPathW GetTempPathW(size win.DWORD, buffer win.LPWSTR) win.DWORD
ext ext_GetFullPathNameW GetFullPathNameW(path win.LPCWSTR, size win.DWORD, buffer win.LPWSTR, filePart win.LPWSTR*) win.DWORD
ext ext_GetFileAttributesW GetFileAttributesW(path win.LPCWSTR) win.DWORD
ext ext_SetFileAttributesW SetFileAttributesW(path win.LPCWSTR, attributes win.DWORD) win.BOOL
ext ext_CreateFileW CreateFileW(path win.LPCWSTR, access win.DWORD, share win.DWORD, security win.LPVOID, disposition win.DWORD, attributes win.DWORD, template win.HANDLE) win.HANDLE
ext ext_GetFinalPathNameByHandleW GetFinalPathNameByHandleW(handle win.HANDLE, buffer win.LPWSTR, size win.DWORD, flags win.DWORD) win.DWORD
ext ext_CloseHandle CloseHandle(handle win.HANDLE) win.BOOL

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
    loop count < 260 && namePtr[count] != 0:
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
    loop this.pending:
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

pub openDir(path str) !$Dir:
    a := ctx.procAlloc
    pattern := try join(path, "*")
    defer pattern.free(a)
    wide := try utf8.utf8To16NT(a, pattern)
    defer slices.free(wide)
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

pub metadata(path str, followLinks bool) !NativeMetadata:
    a := ctx.tempAlloc
    if followLinks:
        resolved := try canonicalize(path)
        defer resolved.free(a)
        ret try metadata(resolved, false)
    ..
    wide := try utf8.utf8To16NT(a, path)
    defer slices.free(wide)
    data := array u8[592]
    handle := ext_FindFirstFileW(slices.toPtr(wide), slices.toPtr(data))
    if cast.ptou(handle) == cast.itou(-1):
        throw errors.native(ext_GetLastError(), "FindFirstFileW failed")
    ..
    defer ext_FindClose(handle)
    ret metadataFromData(slices.toPtr(data))
..

pub setPermissions(path str, permissions u32) !void:
    a := ctx.tempAlloc
    wide := try utf8.utf8To16NT(a, path)
    defer slices.free(wide)
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

pathCall(path str, operation (win.LPCWSTR) win.BOOL, message str) !void:
    a := ctx.tempAlloc
    wide := try utf8.utf8To16NT(a, path)
    defer slices.free(wide)
    if operation(slices.toPtr(wide)) == 0:
        throw errors.native(ext_GetLastError(), message)
    ..
..

pub makeDir(path str) !void:
    a := ctx.tempAlloc
    wide := try utf8.utf8To16NT(a, path)
    defer slices.free(wide)
    if ext_CreateDirectoryW(slices.toPtr(wide), none) == 0:
        throw errors.native(ext_GetLastError(), "CreateDirectoryW failed")
    ..
..

pub removeDir(path str) !void:
    a := ctx.tempAlloc
    try pathCall(a, path, ext_RemoveDirectoryW, "RemoveDirectoryW failed")
..

pub setCurrentDir(path str) !void:
    a := ctx.tempAlloc
    try pathCall(a, path, ext_SetCurrentDirectoryW, "SetCurrentDirectoryW failed")
..

move(source str, destination str, flags u32) !void:
    a := ctx.tempAlloc
    sourceWide := try utf8.utf8To16NT(a, source)
    defer slices.free(sourceWide)
    destinationWide := try utf8.utf8To16NT(a, destination)
    defer slices.free(destinationWide)
    if ext_MoveFileExW(slices.toPtr(sourceWide), slices.toPtr(destinationWide), flags) == 0:
        throw errors.native(ext_GetLastError(), "MoveFileExW failed")
    ..
..

pub rename(source str, destination str) !void:
    a := ctx.tempAlloc
    try move(a, source, destination, 0)
..

pub replace(source str, destination str) !void:
    a := ctx.tempAlloc
    try move(a, source, destination, 9)
..

pub copyFile(source str, destination str) !void:
    a := ctx.tempAlloc
    sourceWide := try utf8.utf8To16NT(a, source)
    defer slices.free(sourceWide)
    destinationWide := try utf8.utf8To16NT(a, destination)
    defer slices.free(destinationWide)
    if ext_CopyFileW(slices.toPtr(sourceWide), slices.toPtr(destinationWide), 0) == 0:
        throw errors.native(ext_GetLastError(), "CopyFileW failed")
    ..
..

wideResult(temp bool) !$str:
    temporary := ctx.tempAlloc
    needed u32 = 0
    if temp:
        needed = ext_GetTempPathW(0, none)
    else:
        needed = ext_GetCurrentDirectoryW(0, none)
    ..
    if needed == 0:
        throw errors.native(ext_GetLastError(), "path query failed")
    ..
    buffer := try temporary.allocT[u16](cast.u32to64(needed) + 1)
    defer temporary.free(buffer)
    written u32 = 0
    if temp:
        written = ext_GetTempPathW(needed + 1, buffer)
    else:
        written = ext_GetCurrentDirectoryW(needed + 1, buffer)
    ..
    if written == 0:
        throw errors.native(ext_GetLastError(), "path query failed")
    ..
    ret try utf8.utf16to8(ctx.procAlloc, slices.fromPtr(buffer, cast.u32to64(written)))
..

pub currentDir() !$str:
    a := ctx.procAlloc
    ret try wideResult(a, false)
..

pub temporaryDir() !$str:
    a := ctx.procAlloc
    ret try wideResult(a, true)
..

pub canonicalize(path str) !$str:
    temporary := ctx.tempAlloc
    wide := try utf8.utf8To16NT(temporary, path)
    defer slices.free(wide)
    handle := ext_CreateFileW(slices.toPtr(wide), 0, 7, none, 3, 0x02000000, none)
    if cast.ptou(handle) == cast.itou(-1):
        throw errors.native(ext_GetLastError(), "CreateFileW failed")
    ..
    defer ext_CloseHandle(handle)
    needed := ext_GetFinalPathNameByHandleW(handle, none, 0, 0)
    if needed == 0:
        throw errors.native(ext_GetLastError(), "GetFinalPathNameByHandleW failed")
    ..
    buffer := try temporary.allocT[u16](cast.u32to64(needed) + 1)
    defer temporary.free(buffer)
    written := ext_GetFinalPathNameByHandleW(handle, buffer, needed + 1, 0)
    if written == 0:
        throw errors.native(ext_GetLastError(), "GetFinalPathNameByHandleW failed")
    ..
    ret try utf8.utf16to8(ctx.procAlloc, slices.fromPtr(buffer, cast.u32to64(written)))
..

pub removeFile(path str) !void:
    a := ctx.tempAlloc
    wide := try utf8.utf8To16NT(a, path)
    defer slices.free(wide)
    if ext_DeleteFileW(slices.toPtr(wide)) == 0:
        throw errors.native(ext_GetLastError(), "DeleteFileW failed")
    ..
..

join(left str, right str) !$str:
    a := ctx.procAlloc
    out := try builder.new()
    defer out.free()
    try out.appendBorrowed(left)
    if left.countBytes() > 0 && strings.byteAt(left, left.countBytes() - 1) != 92 && strings.byteAt(left, left.countBytes() - 1) != 47:
        try out.appendBorrowed("\\")
    ..
    try out.appendBorrowed(right)
    ret try out.build()
..

walkInner(root str, visit (str, bool) !void) !void:
    a := ctx.tempAlloc
    pattern := try join(root, "*")
    defer pattern.free(a)
    widePattern := try utf8.utf8To16NT(a, pattern)
    defer slices.free(widePattern)

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
    loop more:
        nameCount u64 = 0
        loop nameCount < 260 && namePtr[nameCount] != 0:
            nameCount = nameCount + 1
        ..
        nameWide u16[] = slices.fromPtr(namePtr, nameCount)
        name := try utf8.utf16to8(a, nameWide)
        if strings.compare(name, ".") == false && strings.compare(name, "..") == false:
            child := try join(root, name)
            isDirectory bool = (*attributes & 0x10) != 0
            try visit(child, isDirectory)
            if isDirectory:
                try walkInner(child, visit)
            ..
            child.free(a)
        ..
        name.free(a)
        more = ext_FindNextFileW(handle, dataPtr) != 0
    ..
..

pub walk(root str, visit (str, bool) !void) !void:
    a := ctx.tempAlloc
    try walkInner(root, visit)
..
