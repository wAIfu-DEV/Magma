mod path

use "strings.mg" strings
use "cast.mg" cast

@platform("windows")
use "win/path_impl.mg" impl_path

@platform("linux", "android", "ios", "darwin", "freebsd", "netbsd", "openbsd")
use "unix/path_impl.mg" impl_path

pub separator() u8:
    ret impl_path.separator()
..

pub isSeparator(c u8) bool:
    ret c == 47 || c == 92
..

pub isAbsolute(path str) bool:
    n := strings.countBytes(path)
    if n == 0:
        ret false
    ..
    ret impl_path.isAbsolute(path)
..

pub base(path str) str:
    n := strings.countBytes(path)
    end := n
    while end > 0 && isSeparator(strings.byteAt(path, end - 1)):
        end = end - 1
    ..
    start := end
    while start > 0 && isSeparator(strings.byteAt(path, start - 1)) == false:
        start = start - 1
    ..
    p := cast.utop(cast.ptou(strings.toPtr(path)) + start)
    ret strings.fromPtrNoCopy(p, end - start)
..

# Returns the suffix beginning at the final dot in the base name. The base
# before that suffix may be empty, so ".gitignore" has extension ".gitignore".
pub extension(path str) str:
    b := base(path)
    n := strings.countBytes(b)
    i := n
    while i > 0:
        i = i - 1
        if strings.byteAt(b, i) == 46:
            p := cast.utop(cast.ptou(strings.toPtr(b)) + i)
            ret strings.fromPtrNoCopy(p, n - i)
        ..
    ..
    ret strings.fromPtrNoCopy(none, 0)
..
