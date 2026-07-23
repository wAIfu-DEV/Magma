mod path
# Platform-aware lexical path inspection and component extraction.

use "std:strings" strings
use "std:cast" cast
use "std:allocator" allocator

@platform("windows")
use "std:win/path_impl" impl_path

@platform("linux", "android", "ios", "darwin", "freebsd", "netbsd", "openbsd")
use "std:unix/path_impl" impl_path

# Returns the preferred path-separator byte for the current platform.
# @complexity O(1)
# @example
#   separator := path.separator()
pub separator() u8:
    ret impl_path.separator()
..

# Reports whether c is a slash or backslash accepted as a path separator.
# @complexity O(1)
# @example
#   isBoundary := path.isSeparator(character)
pub isSeparator(c u8) bool:
    ret c == 47 || c == 92
..

# Reports whether path is absolute according to current-platform rules.
# @complexity O(1)
# @example
#   absolute := path.isAbsolute("/tmp/data")
pub isAbsolute(path str) bool:
    n := strings.countBytes(path)
    if n == 0:
        ret false
    ..
    ret impl_path.isAbsolute(path)
..

# Allocates the final non-separator component of path.
# Trailing separators are ignored; an all-separator path produces an empty string.
# @complexity O(N)
# @param a allocator for the returned string
# @param path path to inspect
# @returns owned final path component
# @ownership Release the result with the same allocator.
# @example
#   name := try path.base(a, "/tmp/archive.tar")
pub base(a allocator.Allocator, path str) !$str:
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
    ret try strings.fromPtr(a, p, end - start)
..

# Returns the suffix beginning at the final dot in the base name. The base
# before that suffix may be empty, so ".gitignore" has extension ".gitignore".
# @complexity O(N)
# @param a allocator for the returned string
# @param path path to inspect
# @returns owned extension including the dot, or an empty string
# @ownership Release the result with the same allocator.
# @example
#   ext := try path.extension(a, "archive.tar.gz")
pub extension(a allocator.Allocator, path str) !$str:
    b := try base(a, path)
    defer strings.free(a, b)
    n := strings.countBytes(b)
    i := n
    while i > 0:
        i = i - 1
        if strings.byteAt(b, i) == 46:
            p := cast.utop(cast.ptou(strings.toPtr(b)) + i)
            ret try strings.fromPtr(a, p, n - i)
        ..
    ..
    ret try strings.alloc(a, 0)
..
