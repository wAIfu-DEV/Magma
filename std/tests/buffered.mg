mod main

use "std:allocator" allocator
use "std:buffered" buffered
use "std:cast" cast
use "std:errors" errors
use "std:heap" heap
use "std:reader" reader
use "std:strings" strings
use "std:writer" writer

sink(impl ptr, bytes str) !u64:
    total u64* = impl
    count := strings.countBytes(bytes)
    *total = *total + count
    ret count
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
    written u64 = 0
    raw := writer.new(addrof written, sink)
    output := try buffered.writerBuffered(a, raw)
    defer output.close()
    if try output.write("a") != 1 || try output.writeAll("b") != 1 || try output.writeLn("c") != 2:
        throw errors.failure("buffered writer basic writes changed")
    ..
    if try output.writeBool(true) != 4 || try output.writeInt64(-2) != 2 || try output.writeUint64(3) != 1 || try output.writeFloat64(1.5, 1) != 3:
        throw errors.failure("buffered writer formatting changed")
    ..
    facade := output.writer()
    try facade.writeAll("z")
    flushed := try output.flush()
    if flushed != 1 || written != 15:
        throw errors.failure("buffered writer flush changed")
    ..
    calls u64 = 0
    input := reader.new(addrof calls, source)
    bufferedInput := try buffered.readerBuffered(a, input)
    defer bufferedInput.close()
    if bufferedInput.filledCount() != 0 || bufferedInput.isEof():
        throw errors.failure("new buffered reader state changed")
    ..
    if try bufferedInput.fillBuffer() == false || bufferedInput.filledCount() != 2:
        throw errors.failure("buffered reader fill changed")
    ..
    line := try bufferedInput.readLn(a)
    defer strings.free(a, line)
    linePtr u8* = strings.toPtr(line)
    if linePtr[strings.countBytes(line)] != 0:
        throw errors.failure("buffered line is not null terminated")
    ..
    rawReader := bufferedInput.reader()
    spare := array u8[1]
    if try rawReader.readToBuff(spare, 1) != 0:
        throw errors.failure("buffered reader EOF changed")
    ..
    bufferedInput.setFilled(0)
    bufferedInput.markEof()
    if bufferedInput.filledCount() != 0 || bufferedInput.isEof() == false:
        throw errors.failure("buffered reader state controls changed")
    ..
..
