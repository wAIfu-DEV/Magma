mod main

use "../allocator.mg" allocator
use "../errors.mg" errors
use "../heap.mg" heap
use "../list.mg" list
use "../cast.mg" cast

pub main() !void:
    a allocator.Allocator = heap.allocator()
    values := try list.new[u64](a, none)
    defer values.free()
    try values.pushRight(4)
    try values.pushLeft(2)
    taken := try values.popRight()
    if taken != 4 || values.count() != 1:
        throw errors.failure("list behavior changed")
    ..
..
