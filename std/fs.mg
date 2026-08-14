mod fs
# Portable whole-file operations and recursive directory traversal.

use "std:allocator" alc
use "std:file" file
use "std:strings" strings
use "std:errors" errors
use "std:path" path_util
use "std:iterator" iterator

@platform("windows")
use "std:win/fs_impl" impl_fs

@platform("linux", "android", "ios", "darwin", "freebsd", "netbsd", "openbsd")
use "std:unix/fs_impl" impl_fs

# Reads an entire file into an owned string.
# @complexity O(N), where N is the file size
# @param a allocator for the returned contents
# @param path file to read
# @returns owned file contents
# @ownership Release the result with the same allocator.
# @example
#   contents := try fs.readFile(a, "settings.json")
pub readFile(a alc.Allocator, path str) !$str:
    mode := file.mode()
    mode = mode.read()
    f := try file.open(a, path, mode)
    defer f.close()
    count := try f.count()
    r := try f.reader()
    ret try r.read(a, count)
..

# Replaces a file with the complete contents, creating it when absent.
# @complexity O(N), where N is the content byte length
# @param a allocator used for platform path conversion
# @param path destination file
# @param contents bytes to write
# @warning Existing contents are truncated.
# @example
#   try fs.writeFile(a, "output.txt", "complete")
pub writeFile(a alc.Allocator, path str, contents str) !void:
    mode := file.mode()
    mode = mode.write().create().truncate()
    f := try file.open(a, path, mode)
    defer f.close()
    w := try f.writer()
    written := try w.write(contents)
    if written != contents.countBytes():
        throw errors.failure("short file write")
    ..
..

# Deletes one file. Directories are rejected.
# @complexity O(1), excluding filesystem cost
# @example
#   try fs.removeFile(a, "obsolete.tmp")
pub removeFile(a alc.Allocator, path str) !void:
    try impl_fs.removeFile(a, path)
..

# Recursively visits every descendant of root. The path passed to visit is
# borrowed and remains valid only for the duration of the callback.
# @complexity O(E), where E is the number of visited entries
# @param a allocator used during traversal
# @param root directory whose descendants are visited
# @param visit callback receiving a borrowed path and directory flag
# @example
#   try fs.walk(a, root, visitEntry)
pub walk(a alc.Allocator, root str, visit (str, bool) !void) !void:
    try impl_fs.walk(a, root, visit)
..

pub const KIND_FILE u8 = 1
pub const KIND_DIR u8 = 2
pub const KIND_SYMLINK u8 = 3
pub const KIND_OTHER u8 = 4

pub FileKind(
    value u8
)

pub FileKind.isFile() bool:
    ret this.value == KIND_FILE
..

pub FileKind.isDir() bool:
    ret this.value == KIND_DIR
..

pub FileKind.isSymbolicLink() bool:
    ret this.value == KIND_SYMLINK
..

pub FileKind.isOther() bool:
    ret this.value == KIND_OTHER
..

pub Permissions(
    bits u32
)

pub Permissions.readable() bool:
    ret (this.bits & 1) != 0
..

pub Permissions.writable() bool:
    ret (this.bits & 2) != 0
..

pub Permissions.executable() bool:
    ret (this.bits & 4) != 0
..

pub Metadata(
    kindValue FileKind
    sizeValue u64
    permissionsValue Permissions
    modifiedValue i64
)

pub Metadata.kind() FileKind:
    ret this.kindValue
..

pub Metadata.size() u64:
    ret this.sizeValue
..

pub Metadata.permissions() Permissions:
    ret this.permissionsValue
..

pub Metadata.modified() i64:
    ret this.modifiedValue
..

pub metadata(a alc.Allocator, path str) !Metadata:
    native := try impl_fs.metadata(a, path, true)
    ret Metadata(
        kindValue=FileKind(value=native.kind),
        sizeValue=native.size,
        permissionsValue=Permissions(bits=native.permissions),
        modifiedValue=native.modified,
    )
..

pub linkMetadata(a alc.Allocator, path str) !Metadata:
    native := try impl_fs.metadata(a, path, false)
    ret Metadata(
        kindValue=FileKind(value=native.kind),
        sizeValue=native.size,
        permissionsValue=Permissions(bits=native.permissions),
        modifiedValue=native.modified,
    )
..

pub setPermissions(a alc.Allocator, path str, permissions Permissions) !void:
    try impl_fs.setPermissions(a, path, permissions.bits)
..

pub Entry(
    nameValue str
    kindValue FileKind
)

pub Entry.name() str:
    ret this.nameValue
..

pub Entry.kind() FileKind:
    ret this.kindValue
..

pub Dir(
    native impl_fs.Dir
)

pub openDir(a alc.Allocator, path str) !$Dir:
    native := try impl_fs.openDir(a, path)
    ret Dir(native=move native)
..

pub Dir.hasData() bool:
    ret this.native.hasData()
..

pub Dir.next() !$Entry:
    nativeEntry := try this.native.next()
    ret Entry(
        nameValue=nativeEntry.name,
        kindValue=FileKind(value=nativeEntry.kind),
    )
..

dirIteratorHasData(implementation ptr, index u64) bool:
    # SAFETY: Dir.iterator stores its live receiver as the iterator context.
    unsafe:
        directory Dir* = implementation
        ret directory.hasData()
    ..
..

dirIteratorNext(implementation ptr, index u64) !Entry:
    # SAFETY: Dir.iterator stores its live receiver as the iterator context.
    unsafe:
        directory Dir* = implementation
        ret try directory.next()
    ..
..

# Returns a generic iterator borrowing this open directory. The Dir must remain
# alive and open until iteration finishes.
pub Dir.iterator() iterator.Iterator[Entry]:
    ret iterator.new[Entry](this, dirIteratorHasData, dirIteratorNext)
..

destr Dir.close() !void:
    try this.native.close()
..

pub WalkOptions(
    followLinks bool
    includeRoot bool
)

walkEntriesInner(a alc.Allocator, root str, options WalkOptions, visit (str, Metadata) !void) !void:
    directory := try openDir(a, root)
    defer directory.close()
    loop directory.hasData():
        entry := try directory.next()
        parts := array str[2]
        parts[0] = root
        parts[1] = entry.name()
        child := try path_util.join(a, parts)
        defer child.free(a)
        info := try linkMetadata(a, child)
        try visit(child, info)
        if info.kind().isDir():
            try walkEntriesInner(a, child, options, visit)
        elif info.kind().isSymbolicLink() && options.followLinks:
            followed := try metadata(a, child)
            if followed.kind().isDir():
                try walkEntriesInner(a, child, options, visit)
            ..
        ..
    ..
..

pub walkWithOptions(a alc.Allocator, root str, options WalkOptions, visit (str, Metadata) !void) !void:
    if options.includeRoot:
        try visit(root, try linkMetadata(a, root))
    ..
    try walkEntriesInner(a, root, options, visit)
..

pub walkDefault(a alc.Allocator, root str, visit (str, Metadata) !void) !void:
    try walkWithOptions(a, root, WalkOptions(followLinks=false, includeRoot=false), visit)
..

pub makeDir(a alc.Allocator, path str) !void:
    try impl_fs.makeDir(a, path)
..

pub makeDirs(a alc.Allocator, path str) !void:
    existing Metadata, existingError error = metadata(a, path)
    if existingError.ok():
        if existing.kind().isDir():
            ret
        ..
        throw errors.invalidArgument("path exists and is not a directory")
    ..
    parent := try path_util.parent(a, path)
    defer parent.free(a)
    if strings.compare(parent, path) == false && strings.compare(parent, ".") == false:
        try makeDirs(a, parent)
    ..
    try makeDir(a, path)
..

pub removeDir(a alc.Allocator, path str) !void:
    try impl_fs.removeDir(a, path)
..

pub removeTree(a alc.Allocator, root str) !void:
    info := try linkMetadata(a, root)
    if info.kind().isDir() == false:
        try removeFile(a, root)
        ret
    ..
    try removeTreeContents(a, root)
    try removeDir(a, root)
..

removeTreeContents(a alc.Allocator, root str) !void:
    directory := try openDir(a, root)
    defer directory.close()
    loop directory.hasData():
        entry := try directory.next()
        parts := array str[2]
        parts[0] = root
        parts[1] = entry.name()
        child := try path_util.join(a, parts)
        defer child.free(a)
        try removeTree(a, child)
    ..
..

pub rename(a alc.Allocator, source str, destination str) !void:
    try impl_fs.rename(a, source, destination)
..

pub replace(a alc.Allocator, source str, destination str) !void:
    try impl_fs.replace(a, source, destination)
..

pub copyFile(a alc.Allocator, source str, destination str) !void:
    sourceInfo := try metadata(a, source)
    try impl_fs.copyFile(a, source, destination)
    try setPermissions(a, destination, sourceInfo.permissions())
..

pub currentDir(a alc.Allocator) !$str:
    ret try impl_fs.currentDir(a)
..

pub setCurrentDir(a alc.Allocator, path str) !void:
    try impl_fs.setCurrentDir(a, path)
..

pub temporaryDir(a alc.Allocator) !$str:
    ret try impl_fs.temporaryDir(a)
..

pub canonicalize(a alc.Allocator, path str) !$str:
    ret try impl_fs.canonicalize(a, path)
..
