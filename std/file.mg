mod file

use "allocator.mg" alc
use "errors.mg"    errors
use "writer.mg"    w
use "reader.mg"    r
use "file_op_mode.mg" fopm
use "cast.mg"      cast

@platform("windows")
use "win/file_impl.mg" impl_file

@platform("linux", "android", "ios", "darwin", "freebsd", "netbsd", "openbsd")
use "unix/file_impl.mg" impl_file

# File handle wrapper and state.
# O(1).
File(
    handle ptr
    openMode fopm.OpenMode
    open bool
)

# Closes the file if open.
# O(1).
File.close() !void:
    if this.open:
        try impl_file.closeFile(this.handle)
        this.open = false
    ..
..

# Writes bytes to an open file handle.
# O(N) for byte count.
write(f File*, bytes str) !u64:
    if f.open == false:
        throw errors.invalidArgument("write to closed file")
    ..
    ret try impl_file.write(f.handle, bytes)
..

# Returns a writer for this file.
# O(1).
File.writer() !w.Writer:
    if this.open == false || this.openMode.w == false:
        throw errors.invalidArgument("file not open in write mode")
    ..
    ret w.new(this, write)
..

# Reads bytes from an open file handle.
# O(N) for byte count.
read(f File*, buff u8[], n u64) !u64:
    if f.open == false:
        throw errors.invalidArgument("read from closed file")
    ..
    ret try impl_file.read(f.handle, buff, n)
..

# Returns a reader for this file.
# O(1).
File.reader() !r.Reader:
    if this.open == false || this.openMode.r == false:
        throw errors.invalidArgument("file not open in read mode")
    ..
    ret r.new(this, read)
..

# Advances the file pointer to the desired position.
File.seek(offset i64, whence u8) !u64:
    if this.open == false:
        throw errors.invalidArgument("seek on closed file")
    ..
    ret try impl_file.seek(this.handle, offset, whence)
..

File.count() !u64:
    if this.open == false:
        throw errors.invalidArgument("count on closed file")
    ..
    position u64 = try impl_file.seek(this.handle, 0, 1)
    count u64, countErr error = impl_file.seek(this.handle, 0, 2)
    if errors.code(countErr) != 0:
        # Best effort restoration; preserve the original seek failure.
        impl_file.seek(this.handle, cast.utoi(position), 0)
        throw countErr
    ..
    try impl_file.seek(this.handle, cast.utoi(position), 0)
    ret count
..

# Opens a file with the provided path and mode.
# Warning: caller must close the file to avoid leaks.
# O(1) aside from path conversion and syscalls.
# @param a allocator to use
# @param path file path
# @param openMode desired open mode
# @returns open file handle
pub open(a alc.Allocator, path str, openMode fopm.OpenMode) !$File:
    handle ptr = try impl_file.openFile(a, path, openMode)

    f File
    f.handle = handle
    f.openMode = openMode
    f.open = true
    ret f
..

pub mode() fopm.OpenMode:
    om fopm.OpenMode
    ret om
..
