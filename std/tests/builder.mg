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
    if value.isEmpty() == false || value.byteCount() != 0:
        throw errors.failure("new builder is not empty")
    ..
    try value.ensureCapacity()
    try value.appendBorrowed("checked ")
    try value.appendCopy("builder")
    owned := try strings.copy(a, "!")
    try value.appendOwned(owned)
    if value.byteCount() != 16 || value.isEmpty():
        throw errors.failure("builder byte count changed")
    ..
    result := try value.build()
    defer strings.free(a, result)
    if strings.compare(result, "checked builder!") == false:
        throw errors.failure("builder behavior changed")
    ..
    resultPtr u8* = strings.toPtr(result)
    if resultPtr[strings.countBytes(result)] != 0:
        throw errors.failure("built string is not null terminated")
    ..
    try value.reset()
    if value.isEmpty() == false || value.byteCount() != 0:
        throw errors.failure("builder reset changed")
    ..
    try value.add("borrowed", false)
    value.releaseCopies()
    try value.reset()
..
