mod main

use "std:allocator" allocator
use "std:builder" builder
use "std:errors" errors
use "std:heap" heap
use "std:strings" strings

pub main() !void:
    a allocator.Allocator = heap.allocator()
    value := try builder.new()
    defer value.free()
    if value.isEmpty() == false || value.byteCount() != 0:
        throw errors.failure("new builder is not empty")
    ..
    try value.ensureCapacity()
    try value.appendBorrowed("checked ")
    try value.appendCopy("builder")
    owned := try strings.copy("!")
    try value.appendOwned(move owned)
    if value.byteCount() != 16 || value.isEmpty():
        throw errors.failure("builder byte count changed")
    ..
    result := try value.build()
    defer result.free(a)
    if strings.compare(result, "checked builder!") == false:
        throw errors.failure("builder behavior changed")
    ..
    resultPtr u8* = strings.toPtr(result)
    # SAFETY: strings.alloc reserves a trailing terminator after countBytes.
    unsafe:
        if resultPtr[result.countBytes()] != 0:
            throw errors.failure("built string is not null terminated")
        ..
    ..
    try value.reset()
    if value.isEmpty() == false || value.byteCount() != 0:
        throw errors.failure("builder reset changed")
    ..
    try value.addBorrowed("borrowed")
    value.releaseCopies()
    try value.reset()
..
