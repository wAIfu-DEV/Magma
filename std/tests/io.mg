mod main
use "../allocator.mg" allocator
use "../heap.mg" heap
use "../io.mg" io
pub main() !void:
    a allocator.Allocator = heap.allocator()
    output := try io.stdout(a)
    try output.writer().writeAll("")
    try output.close()
..
