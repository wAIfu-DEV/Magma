mod fs_impl_unix
# Unix filesystem backend used by the portable fs module.


use "std:c" c
use "std:allocator" allocator
use "std:builder" builder
use "std:cast" cast
use "std:errors" errors
use "std:strings" strings
use "std:slices" slices

ext ext_unlink unlink(path u8*) c.int
ext ext_opendir opendir(path u8*) ptr
ext ext_readdir readdir(directory ptr) ptr
ext ext_closedir closedir(directory ptr) c.int
ext ext_mkdir mkdir(path u8*, mode u32) c.int
ext ext_rmdir rmdir(path u8*) c.int
ext ext_rename rename(source u8*, destination u8*) c.int
ext ext_getcwd getcwd(buffer u8*, size u64) u8*
ext ext_chdir chdir(path u8*) c.int
ext ext_realpath realpath(path u8*, resolved u8*) u8*
ext ext_getenv getenv(name u8*) u8*
ext ext_stat stat(path u8*, data ptr) c.int
ext ext_lstat lstat(path u8*, data ptr) c.int
ext ext_open open(path u8*, flags c.int, mode c.int) c.int
ext ext_close close(fd c.int) c.int
ext ext_read read(fd c.int, data ptr, count u64) i64
ext ext_write write(fd c.int, data ptr, count u64) i64
ext ext_chmod chmod(path u8*, mode u32) c.int

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
    nextEntry ptr
    open bool
    currentName str
    hasCurrent bool
)

Dir.clearCurrent() void:
    # SAFETY: hasCurrent is the occupancy bit for the uniquely owned currentName.
    unsafe:
    if this.hasCurrent:
        this.currentName.free(this.allocator)
        this.hasCurrent = false
      ..
    ..
..

advance(directory Dir*) void:
    # SAFETY: readdir returns a native dirent pointer whose platform layout has
    # a name field at byte 19 for the lifetime of the directory handle.
    unsafe:
    directory.nextEntry = ext_readdir(directory.handle)
    loop directory.nextEntry != none:
        raw u8* = directory.nextEntry
        name u8* = cast.utop(cast.ptou(raw) + 19)
        borrowed := strings.fromCstrNoCopy(name)
        if strings.compare(borrowed, ".") == false && strings.compare(borrowed, "..") == false:
            ret
        ..
        directory.nextEntry = ext_readdir(directory.handle)
      ..
    ..
..

pub openDir(a allocator.Allocator, path str) !$Dir:
    handle := ext_opendir(strings.toCstrNoCopy(path))
    if handle == none:
        throw errors.failure("opendir failed")
    ..
    result := Dir(allocator=a, handle=handle, nextEntry=none, open=true, currentName="", hasCurrent=false)
    advance(addrof result)
    ret move result
..

pub Dir.hasData() bool:
    ret this.open && this.nextEntry != none
..

pub Dir.next() !$Entry:
    # SAFETY: hasData proves nextEntry is a live native dirent; byte 18 is its
    # type field and byte 19 begins its null-terminated name.
    unsafe:
    if this.hasData() == false:
        throw errors.notFound("directory iterator exhausted")
    ..
    raw u8* = this.nextEntry
    name u8* = cast.utop(cast.ptou(raw) + 19)
    this.clearCurrent()
    this.currentName = try strings.copy(this.allocator, strings.fromCstrNoCopy(name))
    this.hasCurrent = true
    kind u8 = 4
    if raw[18] == 8:
        kind = 1
    elif raw[18] == 4:
        kind = 2
    elif raw[18] == 10:
        kind = 3
    ..
    advance(this)
      ret Entry(name=this.currentName, kind=kind)
    ..
..

destr Dir.close() !void:
    this.clearCurrent()
    if this.open:
        if ext_closedir(this.handle) != 0:
            throw errors.failure("closedir failed")
        ..
        this.open = false
        this.nextEntry = none
    ..
..

pub metadata(a allocator.Allocator, path str, followLinks bool) !NativeMetadata:
    # SAFETY: the platform stat calls initialize the 256-byte buffer; offsets
    # 24, 48, and 88 are the audited target ABI fields used below.
    unsafe:
    data := array u8[256]
    result i32 = 0
    if followLinks:
        result = ext_stat(strings.toCstrNoCopy(path), slices.toPtr(data))
    else:
        result = ext_lstat(strings.toCstrNoCopy(path), slices.toPtr(data))
    ..
    if result != 0:
        throw errors.failure("stat failed")
    ..
    mode u32* = cast.utop(cast.ptou(slices.toPtr(data)) + 24)
    sizeValue i64* = cast.utop(cast.ptou(slices.toPtr(data)) + 48)
    modifiedValue i64* = cast.utop(cast.ptou(slices.toPtr(data)) + 88)
    kind u8 = 4
    typeBits := *mode & 0xF000
    if typeBits == 0x8000:
        kind = 1
    elif typeBits == 0x4000:
        kind = 2
    elif typeBits == 0xA000:
        kind = 3
    ..
    permissions u32 = 0
    if (*mode & 0x124) != 0:
        permissions = permissions | 1
    ..
    if (*mode & 0x92) != 0:
        permissions = permissions | 2
    ..
    if (*mode & 0x49) != 0:
        permissions = permissions | 4
    ..
    size u64 = 0
    if *sizeValue > 0:
        size = cast.itou(*sizeValue)
    ..
      ret NativeMetadata(kind=kind, size=size, permissions=permissions, modified=*modifiedValue)
    ..
..

pub setPermissions(a allocator.Allocator, path str, permissions u32) !void:
    mode u32 = 0
    if (permissions & 1) != 0:
        mode = mode | 0x124
    ..
    if (permissions & 2) != 0:
        mode = mode | 0x92
    ..
    if (permissions & 4) != 0:
        mode = mode | 0x49
    ..
    if ext_chmod(strings.toCstrNoCopy(path), mode) != 0:
        throw errors.failure("chmod failed")
    ..
..

pub makeDir(a allocator.Allocator, path str) !void:
    if ext_mkdir(strings.toCstrNoCopy(path), 0x1FF) != 0:
        throw errors.failure("mkdir failed")
    ..
..

pub removeDir(a allocator.Allocator, path str) !void:
    if ext_rmdir(strings.toCstrNoCopy(path)) != 0:
        throw errors.failure("rmdir failed")
    ..
..

pub rename(a allocator.Allocator, source str, destination str) !void:
    if ext_rename(strings.toCstrNoCopy(source), strings.toCstrNoCopy(destination)) != 0:
        throw errors.failure("rename failed")
    ..
..

pub replace(a allocator.Allocator, source str, destination str) !void:
    try rename(a, source, destination)
..

pub copyFile(a allocator.Allocator, source str, destination str) !void:
    input := ext_open(strings.toCstrNoCopy(source), 0, 0)
    if input < 0:
        throw errors.failure("open source failed")
    ..
    output := ext_open(strings.toCstrNoCopy(destination), 0x241, 0x1B6)
    if output < 0:
        ext_close(input)
        throw errors.failure("open destination failed")
    ..
    buffer := array u8[16384]
    done bool = false
    loop done == false:
        count := ext_read(input, slices.toPtr(buffer), 16384)
        if count < 0:
            ext_close(input)
            ext_close(output)
            throw errors.failure("copy read failed")
        elif count == 0:
            done = true
        else:
            offset i64 = 0
            loop offset < count:
                written := ext_write(output, cast.utop(cast.ptou(slices.toPtr(buffer)) + cast.itou(offset)), cast.itou(count - offset))
                if written <= 0:
                    ext_close(input)
                    ext_close(output)
                    throw errors.failure("copy write failed")
                ..
                offset = offset + written
            ..
        ..
    ..
    if ext_close(input) != 0 || ext_close(output) != 0:
        throw errors.failure("copy close failed")
    ..
..

pub currentDir(a allocator.Allocator) !$str:
    buffer := try a.alloc(4096)
    defer a.free(buffer)
    if ext_getcwd(buffer, 4096) == none:
        throw errors.failure("getcwd failed")
    ..
    ret try strings.copy(a, strings.fromCstrNoCopy(buffer))
..

pub setCurrentDir(a allocator.Allocator, path str) !void:
    if ext_chdir(strings.toCstrNoCopy(path)) != 0:
        throw errors.failure("chdir failed")
    ..
..

pub temporaryDir(a allocator.Allocator) !$str:
    value := ext_getenv(strings.toCstrNoCopy("TMPDIR"))
    if value == none:
        ret try strings.copy(a, "/tmp")
    ..
    ret try strings.copy(a, strings.fromCstrNoCopy(value))
..

pub canonicalize(a allocator.Allocator, path str) !$str:
    buffer := try a.alloc(4096)
    defer a.free(buffer)
    if ext_realpath(strings.toCstrNoCopy(path), buffer) == none:
        throw errors.failure("realpath failed")
    ..
    ret try strings.copy(a, strings.fromCstrNoCopy(buffer))
..

pub removeFile(a allocator.Allocator, path str) !void:
    if ext_unlink(strings.toCstrNoCopy(path)) != 0:
        throw errors.failure("unlink failed")
    ..
..

join(a allocator.Allocator, left str, right str) !$str:
    out := try builder.new(a)
    defer out.free()
    try out.appendBorrowed(left)
    if left.countBytes() > 0 && strings.byteAt(left, left.countBytes() - 1) != 47:
        try out.appendBorrowed("/")
    ..
    try out.appendBorrowed(right)
    ret try out.build()
..

walkInner(a allocator.Allocator, root str, visit (str, bool) !void) !void:
    # SAFETY: each readdir result is live until the next call and follows the
    # same audited dirent layout used by Dir.next.
    unsafe:
    directory := ext_opendir(strings.toCstrNoCopy(root))
    if directory == none:
        throw errors.failure("opendir failed")
    ..
    defer ext_closedir(directory)

    entry := ext_readdir(directory)
    loop entry != none:
        raw u8* = entry
        name u8* = cast.utop(cast.ptou(raw) + 19)
        borrowed := strings.fromCstrNoCopy(name)
        if strings.compare(borrowed, ".") == false && strings.compare(borrowed, "..") == false:
            child := try join(a, root, borrowed)
            isDirectory bool = raw[18] == 4
            try visit(child, isDirectory)
            if isDirectory:
                try walkInner(a, child, visit)
            ..
            child.free(a)
        ..
        entry = ext_readdir(directory)
      ..
    ..
..

pub walk(a allocator.Allocator, root str, visit (str, bool) !void) !void:
    try walkInner(a, root, visit)
..
