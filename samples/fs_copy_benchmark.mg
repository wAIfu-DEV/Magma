mod main
# Linux benchmark: the legacy read/write copy loop versus std:fs.copyFile.

use "std:c" as c
use "std:cast" as cast
use "std:errors" as errors
use "std:fs" as fs
use "std:io" as io
use "std:slices" as slices
use "std:strings" as strings
use "std:time" as time

ext ext_open open(path u8*, flags c.int, mode c.int) c.int
ext ext_read read(fd c.int, data ptr, count u64) i64
ext ext_write write(fd c.int, data ptr, count u64) i64
ext ext_close close(fd c.int) c.int

legacyCopy() !void:
    input := ext_open(strings.toCstrNoCopy("/tmp/magma-copy-source"), 0, 0)
    if input < 0: throw errors.failure("open source failed") ..
    output := ext_open(strings.toCstrNoCopy("/tmp/magma-copy-legacy"), 0x241, 0x1B6)
    if output < 0:
        ext_close(input)
        throw errors.failure("open destination failed")
    ..
    buffer := array u8[16384]
    done bool = false
    loop done == false:
        count := ext_read(input, slices.toPtr(buffer), 16384)
        if count < 0: throw errors.failure("legacy read failed") ..
        if count == 0:
            done = true
        else:
            offset i64 = 0
            loop offset < count:
                next := cast.utop(cast.ptou(slices.toPtr(buffer)) + cast.itou(offset))
                written := ext_write(output, next, cast.itou(count - offset))
                if written <= 0: throw errors.failure("legacy write failed") ..
                offset = offset + written
            ..
        ..
    ..
    ext_close(input)
    ext_close(output)
..

pub main() !void:
    start := time.ticks()
    try legacyCopy()
    oldNs := time.ticksToNs(time.elapsedTicks(start))
    start = time.ticks()
    try fs.copyFile("/tmp/magma-copy-source", "/tmp/magma-copy-native")
    newNs := time.ticksToNs(time.elapsedTicks(start))
    out := io.stdoutUnbuffered()
    try out.writeAll("copy bytes=67108864\nlegacy_read_write_ns=")
    try out.writeUint64(oldNs)
    try out.writeAll("\ncopy_file_range_ns=")
    try out.writeUint64(newNs)
    try out.writeAll("\n")
..
