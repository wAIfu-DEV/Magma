mod file

use "allocator.mg" alc
use "errors.mg"    errors
use "writer.mg"    w
use "reader.mg"    r
use "file_op_mode.mg" fopm

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
File.close() void:
    if this.open:
        impl_file.closeFile(this.handle)
        this.open = false
    ..
..

# Writes bytes to an open file handle.
# O(N) for byte count.
write(f File*, bytes str) !u64:
    if f.open == false:
        throw errors.errInvalidArgument("write to closed file")
    ..
    ret try impl_file.write(f.handle, bytes)
..

# Returns a writer for this file.
# O(1).
File.writer() !w.Writer:
    if this.openMode.write == false:
        throw errors.errInvalidArgument("file not open in write mode")
    ..
    ret w.new(this, write)
..

# Reads bytes from an open file handle.
# O(N) for byte count.
read(f File*, buff u8[], n u64) !u64:
    if f.open == false:
        throw errors.errInvalidArgument("read from closed file")
    ..
    ret try impl_file.read(f.handle, buff, n)
..

# Returns a reader for this file.
# O(1).
File.reader() !r.Reader:
    if this.openMode.read == false:
        throw errors.errInvalidArgument("file not open in read mode")
    ..
    ret reader.new(this, read)
..

# Advances the file pointer to the desired position.
File.seek(offset i64, whence u8) !u64:
    if this.open == false:
        throw errors.errInvalidArgument("seek on closed file")
    ..
    ret try impl_file.seek(this.handle, offset, whence)
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

pub modeRead() fopm.OpenMode:
    om fopm.OpenMode
    om.write = false
    om.read = true
    om.append = false
    ret om
..

pub modeWrite() fopm.OpenMode:
    om fopm.OpenMode
    om.write = true
    om.read = false
    om.append = false
    ret om
..

pub modeAppend() fopm.OpenMode:
    om fopm.OpenMode
    om.write = true
    om.read = false
    om.append = true
    ret om
..