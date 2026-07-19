mod main

use "../allocator.mg" allocator
use "../errors.mg" errors
use "../heap.mg" heap
use "../linear_map.mg" linear_map
use "../cast.mg" cast
use "../strings.mg" strings

pub main() !void:
    a allocator.Allocator = heap.allocator()
    values := try linear_map.new[u64](a, none)
    defer values.free()
    try values.set("answer", 42)
    answer := try values.get("answer")
    if answer != 42 || values.count() != 1:
        throw errors.failure("linear map behavior changed")
    ..
    if try values.indexOf("answer") != 0 || strings.compare(values.keysView()[0], "answer") == false || values.valuesView()[0] != 42:
        throw errors.failure("linear map views changed")
    ..
    taken := try values.take("answer")
    if taken != 42 || values.count() != 0:
        throw errors.failure("linear map take changed")
    ..
    try values.grow()
    try values.clear()
    if values.count() != 0:
        throw errors.failure("linear map clear changed")
    ..
    try values.set("answer", 42)
    try values.delete("answer")
..
