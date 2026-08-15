mod main
use "std:duplex" duplex
use "std:errors" errors

Stream impl duplex.Duplex(value u8)

Stream.write(bytes str) !u64:
    ret bytes.countBytes()
..

Stream.readRaw(bytes u8[], count u64) !u64:
    ret 0
..

pub main() !void:
    implementation := Stream(value=0)
    stream := implementation.proto[duplex.Duplex]()
    count := try stream.writer().write("ok")
    if count != 2:
        throw errors.failure("duplex behavior changed")
    ..
    buffer := array u8[1]
    if try stream.reader().readToBuff(buffer, 1) != 0:
        throw errors.failure("duplex reader changed")
    ..
..
