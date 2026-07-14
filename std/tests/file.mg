mod main
use "../allocator.mg" allocator
use "../file.mg" file
use "../heap.mg" heap
pub main() !void:
    a allocator.Allocator = heap.allocator()
    output := try file.open(a, "std_checked_test_file.tmp", file.mode().write())
    try output.writer().writeAll("checked")
    try output.close()
..
