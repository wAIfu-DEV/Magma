mod main

use "../allocator.mg" allocator
use "../builder.mg" builder
use "../errors.mg" errors
use "../heap.mg" heap
use "../strings.mg" strings

pub main() !void:
    a allocator.Allocator = heap.allocator()
    value := try builder.new(a)
    defer value.free()
    try value.appendBorrowed("checked ")
    try value.appendCopy("builder")
    result := try value.build()
    defer strings.free(a, result)
    if strings.compare(result, "checked builder") == false:
        throw errors.failure("builder behavior changed")
    ..
..
