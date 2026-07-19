mod main

use "../allocator.mg" allocator
use "../errors.mg" errors
use "../fake_alloc.mg" fake_alloc

pub main() !void:
    a allocator.Allocator = fake_alloc.allocator()
    value u8*, allocErr error = a.alloc(1)
    if value != none || errors.code(allocErr) != errors.code(errors.failure("")):
        throw errors.failure("fake allocator did not reject allocation")
    ..

    resized u8*, reallocErr error = a.realloc(none, 1)
    if resized != none || errors.code(reallocErr) != errors.code(errors.failure("")):
        throw errors.failure("fake allocator did not reject reallocation")
    ..
    a.free(none)
..
