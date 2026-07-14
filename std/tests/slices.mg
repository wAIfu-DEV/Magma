mod main
use "../allocator.mg" allocator
use "../errors.mg" errors
use "../heap.mg" heap
use "../slices.mg" slices
pub main() !void:
    a allocator.Allocator = heap.allocator()
    block := try a.alloc(4)
    view := slices.fromPtr(block, 4)
    if slices.count(view) != 4 || slices.toPtr(view) != block:
        throw errors.failure("slice behavior changed")
    ..
    slices.free(a, view)
..
