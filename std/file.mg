mod file

use "allocator.mg" alc
use "errors.mg"    errors
use "writer.mg"    w
use "reader.mg"    r

@platform("windows")
use "win/file_impl.mg" impl_file

@platform("linux", "android", "ios", "darwin", "freebsd", "netbsd", "openbsd")
use "unix/file_impl.mg" impl_file

OpenMode(
    read bool
    write bool
    append bool
)

File(
    handle ptr
    openMode OpenMode
    open bool
)

File.close() void:
    if this.open:
        impl_file.closeFile(this.handle)
        this.open = false
    ..
..

write(f File*, bytes str) !u64:
    if f.open == false:
        throw errors.errInvalidArgument("write to closed file")
    ..
    ret try impl_file.write(f.handle, bytes)
..

File.writer() !w.Writer:
    if this.openMode.write == false:
        throw errors.errInvalidArgument("file not open in write mode")
    ..
    wr w.Writer
    wr.impl = this
    wr.fn_write = write
    ret wr
..

read(f File*, buff u8[], n u64) !u64:
    if f.open == false:
        throw errors.errInvalidArgument("read from closed file")
    ..
    ret try impl_file.read(f.handle, buff, n)
..

File.reader() !r.Reader:
    if this.openMode.read == false:
        throw errors.errInvalidArgument("file not open in read mode")
    ..
    rr r.Reader
    rr.impl = this
    rr.fn_read = read
    ret rr
..

pub open(a alc.Allocator, path str, openMode OpenMode) !$File:
    handle ptr = try impl_file.openFile(a, path, openMode)

    f File
    f.handle = handle
    f.openMode = openMode
    f.open = true
    ret f
..
