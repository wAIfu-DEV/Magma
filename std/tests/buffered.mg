mod main

use "../allocator.mg" allocator
use "../buffered.mg" buffered
use "../cast.mg" cast
use "../errors.mg" errors
use "../heap.mg" heap
use "../reader.mg" reader
use "../strings.mg" strings
use "../writer.mg" writer

sink(impl ptr, bytes str) !u64:
    ret 0
..

source(impl ptr, bytes u8[], count u64) !u64:
    calls u64* = impl
    if *calls == 0 && count >= 2:
        bytes[0] = 65
        bytes[1] = 10
        *calls = 1
        ret 2
    ..
    ret 0
..

pub main() !void:
    a allocator.Allocator = heap.allocator()
    raw := writer.new(none, sink)
    output := try buffered.writerBuffered(a, raw)
    try output.writer().writeAll("")
    try output.close()

    calls u64 = 0
    input := reader.new(addrof calls, source)
    bufferedInput := try buffered.readerBuffered(a, input)
    defer bufferedInput.close()
    line := try bufferedInput.readLn(a)
    defer strings.free(a, line)
    linePtr u8* = strings.toPtr(line)
    if linePtr[strings.countBytes(line)] != 0:
        throw errors.failure("buffered line is not null terminated")
    ..
..
