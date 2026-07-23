mod main
use "std:allocator" allocator
use "std:errors" errors
use "std:fs" fs
use "std:heap" heap
use "std:strings" strings

found bool

visit(path str, isDirectory bool) !void:
    if strings.compare(path, "std\\fs.mg") || strings.compare(path, "std/fs.mg"):
        found = true
    ..
..

pub main() !void:
    a allocator.Allocator = heap.allocator()
    try fs.writeFile(a, "std_checked_test_fs.tmp", "checked fs")
    contents := try fs.readFile(a, "std_checked_test_fs.tmp")
    defer strings.free(a, contents)
    if strings.compare(contents, "checked fs") == false:
        throw errors.failure("filesystem behavior changed")
    ..
    try fs.walk(a, "std", visit)
    if found == false:
        throw errors.failure("directory walk did not find std/fs.mg")
    ..
    try fs.removeFile(a, "std_checked_test_fs.tmp")
    missing str, missingErr error = fs.readFile(a, "std_checked_test_fs.tmp")
    if missingErr.ok():
        strings.free(a, missing)
        throw errors.failure("removed file can still be opened")
    ..
..
