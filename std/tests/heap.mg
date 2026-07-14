mod main
use "../heap.mg" heap
pub main() !void:
    block := try heap.allocZero(32)
    block = try heap.reallocZero(block, 64, 32)
    heap.free(block)
..
