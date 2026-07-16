mod main

use "../allocator.mg" allocator
use "../array.mg" array
use "../cast.mg" cast
use "../errors.mg" errors
use "../heap.mg" heap

pub main() !void:
    a allocator.Allocator = heap.allocator()
    values := try array.new[u64](a, none)
    defer values.free(a)

    try values.pushRight(a, 10)
    try values.pushLeft(a, 5)
    if values.count() != 2:
        throw errors.failure("array count changed")
    ..

    taken := try values.popRight(a)
    if taken != 10:
        throw errors.failure("array pop changed")
    ..
..
