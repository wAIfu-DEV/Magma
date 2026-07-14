mod main

use "../allocator.mg" allocator
use "../buffered.mg" buffered
use "../cast.mg" cast
use "../heap.mg" heap
use "../writer.mg" writer

sink(impl ptr, bytes str) !u64:
    ret 0
..

pub main() !void:
    a allocator.Allocator = heap.allocator()
    raw := writer.new(cast.utop(0), sink)
    output := try buffered.writerBuffered(a, raw)
    try output.writer().writeAll("")
    try output.close()
..
