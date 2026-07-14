mod main

use "../allocator.mg" allocator
use "../errors.mg" errors
use "../heap.mg" heap
use "../json.mg" json

cleanup(value $json.Value) void:
..

pub main() !void:
    a allocator.Allocator = heap.allocator()
    object := try json.newObject(a, cleanup)
    defer object.free()
    try object.set("answer", json.numberInt(42))
    value := try object.get("answer")
    answer := try value.asInt()
    if answer != 42 || object.count() != 1:
        throw errors.failure("json object behavior changed")
    ..

    array := try json.newArray(a, cleanup)
    defer array.free()
    try array.append(json.boolean(true))
    if array.count() != 1:
        throw errors.failure("json array behavior changed")
    ..
..
