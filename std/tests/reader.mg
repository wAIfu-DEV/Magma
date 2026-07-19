mod main
use "../allocator.mg" allocator
use "../cast.mg" cast
use "../errors.mg" errors
use "../heap.mg" heap
use "../reader.mg" reader
use "../strings.mg" strings
source(impl ptr, bytes u8[], count u64) !u64:
    if count > 0:
        bytes[0] = 65
        ret 1
    ..
    ret 0
..
pub main() !void:
    a allocator.Allocator = heap.allocator()
    input := reader.new(none, source)
    result := try input.read(a, 1)
    defer strings.free(a, result)
    if strings.compare(result, "A") == false:
        throw errors.failure("reader behavior changed")
    ..
    resultPtr u8* = strings.toPtr(result)
    if resultPtr[strings.countBytes(result)] != 0:
        throw errors.failure("read string is not null terminated")
    ..
    buffer u8[2]
    readCount := try input.readToBuff(buffer, 2)
    if readCount != 1 || buffer[0] != 65:
        throw errors.failure("reader readToBuff changed")
    ..
..
