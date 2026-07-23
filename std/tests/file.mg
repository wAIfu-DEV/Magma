mod main
use "std:allocator" allocator
use "std:file" file
use "std:heap" heap
use "std:errors" errors
pub main() !void:
    a allocator.Allocator = heap.allocator()
    output := try file.open(a, "std_checked_test_file.tmp", file.mode().read().write().create().truncate())
    outputWriter := try output.writer()
    try outputWriter.writeAll("checked")
    if try output.count() != 7 || try output.seek(0, 0) != 0:
        try output.close()
        throw errors.failure("file count or seek changed")
    ..
    outputReader := try output.reader()
    buffer := array u8[7]
    if try outputReader.readToBuff(buffer, 7) != 7 || buffer[0] != 99 || buffer[6] != 100:
        try output.close()
        throw errors.failure("file read changed")
    ..
    try output.close()

    # Write access alone must preserve the existing file and must not imply
    # truncation. Opening with truncate makes that destructive intent explicit.
    preserving := try file.open(a, "std_checked_test_file.tmp", file.mode().write())
    preservingWriter := try preserving.writer()
    try preservingWriter.writeAll("x")
    try preserving.close()
    preserved := try file.open(a, "std_checked_test_file.tmp", file.mode().read())
    if try preserved.count() != 7:
        try preserved.close()
        throw errors.failure("write mode unexpectedly truncated the file")
    ..
    try preserved.close()

    truncating := try file.open(a, "std_checked_test_file.tmp", file.mode().write().truncate())
    if try truncating.count() != 0:
        try truncating.close()
        throw errors.failure("truncate mode did not clear the file")
    ..
    try truncating.close()
..
