mod main

use "std:allocator" allocator
use "std:errors" errors
use "std:hash_map" hash_map
use "std:heap" heap
use "std:cast" cast

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
