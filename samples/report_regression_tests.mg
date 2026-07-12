mod main

use "../std/heap.mg" heap
use "../std/memory.mg" memory
use "../std/slices.mg" slices
use "../std/strings.mg" strings
use "../std/utf8.mg" utf8
use "../std/writer.mg" writer
use "../std/reader.mg" reader
use "../std/buffered.mg" buffered
use "../std/errors.mg" errors
use "../std/cast.mg" cast
use "../std/linear_map.mg" linear_map

Capture(
    data u8*
    count u64
    maxChunk u64
)

captureWrite(c Capture*, bytes str) !u64:
    n u64 = strings.countBytes(bytes)
    if n > c.maxChunk:
        n = c.maxChunk
    ..
    destination ptr = cast.utop(cast.ptou(c.data) + c.count)
    memory.copy(strings.toPtr(bytes), destination, n)
    c.count = c.count + n
    ret n
..

invalidRead(impl ptr, buff u8[], n u64) !u64:
    ret n + 1
..

assertBytes(actual u8*, expected str) !void:
    if memory.compare(actual, strings.toPtr(expected), strings.countBytes(expected)) == false:
        throw errors.failure("byte comparison failed")
    ..
..

main() !void:
    a := heap.allocator()

    # Backward, forward, and zero-length overlapping moves.
    bytes u8[6]
    bytes[0] = 0
    bytes[1] = 1
    bytes[2] = 2
    bytes[3] = 3
    bytes[4] = 4
    bytes[5] = 5
    p u8* = slices.toPtr(bytes)
    memory.move(p, cast.utop(cast.ptou(p) + 1), 5)
    if bytes[1] != 0 || bytes[5] != 4:
        throw errors.failure("backward memory.move failed")
    ..
    memory.move(cast.utop(cast.ptou(p) + 1), p, 5)
    if bytes[0] != 0 || bytes[4] != 4:
        throw errors.failure("forward memory.move failed")
    ..
    memory.move(p, p, 0)

    # The logical UTF-16 view excludes the terminator, which remains available
    # immediately after it for native APIs.
    wide := try utf8.utf8To16NT(a, "A")
    widePtr u16* = slices.toPtr(wide)
    if slices.count(wide) != 1 || widePtr[0] != 65 || widePtr[1] != 0:
        throw errors.failure("utf8To16NT terminator failed")
    ..
    a.free(widePtr)

    captureData u8* = try a.alloc(128)
    defer a.free(captureData)
    capture Capture
    capture.data = captureData
    capture.maxChunk = 2
    out := writer.new(addrof capture, captureWrite)

    try out.writeAll("partial")
    try assertBytes(capture.data, "partial")

    capture.count = 0
    min i64 = -9223372036854775807 - 1
    try out.writeInt64(min)
    try assertBytes(capture.data, "-9223372036854775808")

    capture.count = 0
    bw := try buffered.writerBuffered(a, out)
    bufferedOut := bw.writer()
    try bufferedOut.write("buffered")
    try bw.flush()
    if bw.position != 0:
        throw errors.failure("buffered flush retained data")
    ..
    try assertBytes(capture.data, "buffered")
    try bw.close()

    badReader := reader.new(capture.data, invalidRead)
    one u8[1]
    ignored u64, readErr error = badReader.readToBuff(one, 1)
    if errors.code(readErr) == 0:
        throw errors.failure("reader accepted an invalid byte count")
    ..

    map := try linear_map.new[u64](a)
    try map.set("one", 1)
    try map.set("two", 2)
    try map.delete("one")
    two := try map.get("two")
    if two != 2:
        throw errors.failure("linear map became inconsistent")
    ..
    try map.clear()
    map.free()
..
