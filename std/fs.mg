mod fs

use "allocator.mg" alc
use "file.mg" file
use "strings.mg" strings
use "errors.mg" errors

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
