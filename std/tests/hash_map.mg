mod main

use "../allocator.mg" allocator
use "../errors.mg" errors
use "../hash_map.mg" hash_map
use "../heap.mg" heap
use "../cast.mg" cast

pub main() !void:
    a allocator.Allocator = heap.allocator()
    values := try hash_map.new[u64](a, 8, none)
    defer values.free()
    try values.set("answer", 42)
    try values.set("answer", 43)
    answer := try values.get("answer")
    if answer != 43 || values.count() != 1 || try values.indexOf("answer") >= 8:
        throw errors.failure("hash map behavior changed")
    ..
    try values.resize(16)
    taken := try values.take("answer")
    if taken != 43 || values.count() != 0:
        throw errors.failure("hash map resize or take changed")
    ..
    try values.set("answer", 44)
    try values.delete("answer")
..
