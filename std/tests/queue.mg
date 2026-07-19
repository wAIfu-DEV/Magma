mod main

use "../allocator.mg" allocator
use "../errors.mg" errors
use "../heap.mg" heap
use "../queue.mg" queue
use "../cast.mg" cast

pub main() !void:
    a allocator.Allocator = heap.allocator()
    values := try queue.new[u64](a, none)
    defer values.free()
    try values.enqueue(3)
    try values.enqueue(7)
    first := try values.dequeue()
    if first != 3 || values.count() != 1:
        throw errors.failure("queue behavior changed")
    ..
    if values.view()[0] != 7:
        throw errors.failure("queue view changed")
    ..
    try values.clear()
    if values.count() != 0:
        throw errors.failure("queue clear changed")
    ..
..
