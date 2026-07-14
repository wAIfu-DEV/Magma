mod main
use "../win/heap_impl.mg" heap_impl
pub main() !void:
    block := try heap_impl.allocZero(8)
    heap_impl.free(block)
..
