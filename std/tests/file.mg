mod main
use "../allocator.mg" allocator
use "../file.mg" file
use "../heap.mg" heap
use "../errors.mg" errors
pub main() !void:
    a allocator.Allocator = heap.allocator()
    output := try file.open(a, "std_checked_test_file.tmp", file.mode().read().write())
    outputWriter := try output.writer()
    try outputWriter.writeAll("checked")
    if try output.count() != 7 || try output.seek(0, 0) != 0:
        try output.close()
        throw errors.failure("file count or seek changed")
    ..
    outputReader := try output.reader()
    buffer u8[7]
    if try outputReader.readToBuff(buffer, 7) != 7 || buffer[0] != 99 || buffer[6] != 100:
        try output.close()
        throw errors.failure("file read changed")
    ..
    try output.close()
..
