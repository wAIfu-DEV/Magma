mod fs
# Portable whole-file operations and recursive directory traversal.

use "std:allocator" alc
use "std:file" file
use "std:strings" strings
use "std:errors" errors

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
    if written != strings.countBytes(contents):
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
