mod main
use "../allocator.mg" allocator
use "../errors.mg" errors
use "../heap.mg" heap
use "../strings.mg" strings
pub main() !void:
    a allocator.Allocator = heap.allocator()
    copy := try strings.copy(a, "magma")
    defer strings.free(a, copy)
    if strings.countBytes(copy) != 5 || strings.compare(copy, "magma") == false:
        throw errors.failure("strings behavior changed")
    ..
..
