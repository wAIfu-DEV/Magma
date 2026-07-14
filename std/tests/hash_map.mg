mod main

use "../allocator.mg" allocator
use "../errors.mg" errors
use "../hash_map.mg" hash_map
use "../heap.mg" heap
use "../cast.mg" cast

pub main() !void:
    a allocator.Allocator = heap.allocator()
    values := try hash_map.new[u64](a, 8, cast.utop(0))
    defer values.free()
    try values.set("answer", 42)
    try values.set("answer", 43)
    answer := try values.get("answer")
    if answer != 43 || values.count() != 1:
        throw errors.failure("hash map behavior changed")
    ..
    try values.delete("answer")
..
