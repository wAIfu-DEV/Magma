mod fs

use "allocator.mg" alc
use "file.mg" file
use "strings.mg" strings
use "errors.mg" errors

@platform("windows")
use "win/fs_impl.mg" impl_fs

@platform("linux", "android", "ios", "darwin", "freebsd", "netbsd", "openbsd")
use "unix/fs_impl.mg" impl_fs

pub readFile(a alc.Allocator, path str) !$str:
    mode := file.mode()
    mode = mode.read()
    f := try file.open(a, path, mode)
    defer f.close()
    count := try f.count()
    r := try f.reader()
    ret try r.read(a, count)
..

pub writeFile(a alc.Allocator, path str, contents str) !void:
    mode := file.mode()
    mode = mode.write()
    f := try file.open(a, path, mode)
    defer f.close()
    w := try f.writer()
    written := try w.write(contents)
    if written != strings.countBytes(contents):
        throw errors.failure("short file write")
    ..
..

# Deletes one file. Directories are rejected.
pub removeFile(a alc.Allocator, path str) !void:
    try impl_fs.removeFile(a, path)
..

# Recursively visits every descendant of root. The path passed to visit is
# borrowed and remains valid only for the duration of the callback.
pub walk(a alc.Allocator, root str, visit (str, bool) !void) !void:
    try impl_fs.walk(a, root, visit)
..
