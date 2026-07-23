mod file
# Portable file handles with reader, writer, seeking, and explicit cleanup.

use "std:allocator" alc
use "std:errors"    errors
use "std:writer"    w
use "std:reader"    r
use "std:file_op_mode" fopm
use "std:cast"      cast

@platform("windows")
use "std:win/file_impl" impl_file

@platform("linux", "android", "ios", "darwin", "freebsd", "netbsd", "openbsd")
use "std:unix/file_impl" impl_file

# File handle wrapper and state.
# @complexity O(1).
pub File(
    handle ptr
    openMode fopm.OpenMode
    open bool
)

# Closes the file if open.
# @complexity O(1).
# @example
#   try handle.close()
destr File.close() !void:
    if this.open:
        try impl_file.closeFile(this.handle)
        this.open = false
    ..
..

# Writes bytes to an open file handle.
# @complexity O(N) for byte count.
write(f File*, bytes str) !u64:
    if f.open == false:
        throw errors.invalidArgument("write to closed file")
    ..
    ret try impl_file.write(f.handle, bytes)
..

# Returns a writer for this file.
# @complexity O(1).
# @throws invalidArgument when the file is closed or lacks write access
# @ownership The returned writer borrows the File, which must remain open.
# @example
#   output := try handle.writer()
File.writer() !w.Writer:
    if this.open == false || this.openMode.w == false:
        throw errors.invalidArgument("file not open in write mode")
    ..
    ret w.new(this, write)
..

# Reads bytes from an open file handle.
# @complexity O(N) for byte count.
read(f File*, buff u8[], n u64) !u64:
    if f.open == false:
        throw errors.invalidArgument("read from closed file")
    ..
    ret try impl_file.read(f.handle, buff, n)
..

# Returns a reader for this file.
# @complexity O(1).
# @throws invalidArgument when the file is closed or lacks read access
# @ownership The returned reader borrows the File, which must remain open.
# @example
#   input := try handle.reader()
File.reader() !r.Reader:
    if this.open == false || this.openMode.r == false:
        throw errors.invalidArgument("file not open in read mode")
    ..
    ret r.new(this, read)
..

# Advances the file pointer to the desired position.
# whence is 0 for start, 1 for current position, or 2 for end.
# @complexity O(1), excluding platform syscall cost
# @returns resulting absolute byte position
# @throws invalidArgument when the file is closed
# @example
#   position := try handle.seek(0, 0)
File.seek(offset i64, whence u8) !u64:
    if this.open == false:
        throw errors.invalidArgument("seek on closed file")
    ..
    ret try impl_file.seek(this.handle, offset, whence)
..

# Returns the file size in bytes without changing the current position.
# @complexity O(1), excluding platform syscall cost
# @throws invalidArgument when the file is closed
# @example
#   byteCount := try handle.count()
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
# @warning caller must close the file to avoid leaks.
# @complexity O(1) aside from path conversion and syscalls.
# @param a allocator to use
# @param path file path
# @param openMode desired open mode
# @returns open file handle
# @mustcall close
# @example
#   handle := try file.open(a, "data.bin", file.mode().read())
pub open(a alc.Allocator, path str, openMode fopm.OpenMode) !$File:
    handle ptr = try impl_file.openFile(a, path, openMode)
    ret File(handle=handle, openMode=openMode, open=true)
..

# Returns an empty mode that can be configured through chainable methods.
# @complexity O(1)
# @example
#   openMode := file.mode().write().create().truncate()
pub mode() fopm.OpenMode:
    ret fopm.OpenMode(r=false, w=false, a=false, c=false, t=false)
..
