mod file

use "allocator.mg" alc
use "errors.mg"    errors

@platform("windows")
use "win/file_impl.mg" impl_file

@platform("unix")
use "unix/file_impl.mg" impl_file

OpenMode(
    read bool
    write bool
    append bool
)

File(
    handle ptr,
    open bool,
)

# it is not yet clear if variables not explicitly initialized with a constructor
# should also get to have a destructor call.
# For now every destructors should be able to handle zero-initialized data.
# (True as of 2026-01-16 at least)

File.destructor() void:
    this.close()
..

File.close() void:
    if this.open:
        impl_file.closeFile(this.handle)
        this.open = false
    ..
..

File.write(bytes str) !u64:
    if this.open == false:
        throw errors.errInvalidArgument("write on closed file")
    ..
    ret try impl_file.write(this.handle, bytes)
..

File.read(n u64) !str:
    if this.open == false:
        throw errors.errInvalidArgument("read from closed file")
    ..
    ret try impl_file.read(this.handle, n)
..

pub openFile(a alc.Allocator, path str, openMode OpenMode) !File:
    handle ptr = try impl_file.openFile(a, path, openMode)

    f File
    f.handle = handle
    f.open = true
    ret f
..
