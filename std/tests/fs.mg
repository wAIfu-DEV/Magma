mod main
use "../allocator.mg" allocator
use "../errors.mg" errors
use "../fs.mg" fs
use "../heap.mg" heap
use "../strings.mg" strings
pub main() !void:
    a allocator.Allocator = heap.allocator()
    try fs.writeFile(a, "std_checked_test_fs.tmp", "checked fs")
    contents := try fs.readFile(a, "std_checked_test_fs.tmp")
    defer strings.free(a, contents)
    if strings.compare(contents, "checked fs") == false:
        throw errors.failure("filesystem behavior changed")
    ..
..
