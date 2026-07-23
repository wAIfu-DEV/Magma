mod main
use "std:allocator" allocator
use "std:heap" heap
pub main() !void:
    a allocator.Allocator = heap.allocator()
    block := try a.alloc(16)
    block = try a.realloc(block, 32)
    a.free(block)
..
