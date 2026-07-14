mod main

use "../allocator.mg" allocator
use "../errors.mg" errors
use "../heap.mg" heap
use "../linear_map.mg" linear_map
use "../cast.mg" cast

pub main() !void:
    a allocator.Allocator = heap.allocator()
    values := try linear_map.new[u64](a, cast.utop(0))
    defer values.free()
    try values.set("answer", 42)
    answer := try values.get("answer")
    if answer != 42 || values.count() != 1:
        throw errors.failure("linear map behavior changed")
    ..
    try values.delete("answer")
..
