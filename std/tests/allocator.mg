mod main
use "../allocator.mg" allocator
use "../heap.mg" heap
pub main() !void:
    a allocator.Allocator = heap.allocator()
    block := try a.alloc(16)
    block = try a.realloc(block, 32)
    a.free(block)
..
