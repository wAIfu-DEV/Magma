mod main
use "../allocator.mg" allocator
use "../errors.mg" errors
use "../heap.mg" heap
use "../slices.mg" slices
pub main() !void:
    a allocator.Allocator = heap.allocator()
    empty := slices.fromPtr(none, 0)
    if slices.count(empty) != 0:
        throw errors.failure("empty slice construction changed")
    ..
    view := try slices.alloc[u8](a, 4)
    block := slices.toPtr(view)
    if slices.count(view) != 4 || slices.toPtr(view) != block:
        throw errors.failure("slice behavior changed")
    ..
    words := slices.reinterpret[u8, u16](view)
    if slices.count(words) != 2:
        slices.free(a, view)
        throw errors.failure("slice reinterpret changed")
    ..
    slices.free(a, view)
..
