mod main

use "../allocator.mg" allocator
use "../errors.mg" errors
use "../hash_map.mg" hash_map
use "../heap.mg" heap

Resource(
    value u64
)

destroyed u64

destr Resource.free() void:
    this.value = 0
    destroyed = destroyed + 1
..

make(value u64) $Resource:
    resource Resource
    resource.value = value
    ret resource
..

cleanup(value $Resource) void:
    value.free()
..

inspect(value Resource) u64:
    ret value.value
..

pub main() !void:
    a allocator.Allocator = heap.allocator()
    values := try hash_map.new[Resource](a, 8, cleanup)
    defer values.free()

    try values.set("resource", make(42))
    try values.set("resource", make(43))
    borrowed := try values.get("resource")
    if inspect(borrowed) != 43:
        throw errors.failure("borrowed get changed")
    ..

    owned := try values.take("resource")
    owned.free()
    if destroyed != 2:
        throw errors.failure("container cleanup changed")
    ..
..
