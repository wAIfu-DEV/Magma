mod main
# Path-based range read: generic open/seek/read/close versus fs.readRange.

use "std:errors" errors
use "std:file" file
use "std:fs" fs
use "std:heap" heap
use "std:io" io
use "std:time" time

const ITERATIONS u64 = 10000

legacy() !u64:
    total u64 = 0
    for i u64 = 0 to ITERATIONS:
        mode := file.mode()
        mode = mode.read()
        value := try file.open("/tmp/magma-range-source", mode)
        try value.seek(1024, 0)
        reader := try value.reader()
        bytes := try reader.read(64)
        total = total + bytes.countBytes()
        bytes.free(heap.allocator())
        try value.close()
    ..
    ret total
..

positional() !u64:
    total u64 = 0
    for i u64 = 0 to ITERATIONS:
        bytes := try fs.readRange("/tmp/magma-range-source", 1024, 64)
        total = total + bytes.countBytes()
        bytes.free(heap.allocator())
    ..
    ret total
..

pub main() !void:
    start := time.ticks()
    oldTotal := try legacy()
    oldNs := time.ticksToNs(time.elapsedTicks(start))
    start = time.ticks()
    newTotal := try positional()
    newNs := time.ticksToNs(time.elapsedTicks(start))
    if oldTotal != newTotal || newTotal != ITERATIONS * 64:
        throw errors.failure("range benchmark result changed")
    ..
    out := io.stdoutUnbuffered()
    try out.writeAll("range reads=")
    try out.writeUint64(ITERATIONS)
    try out.writeAll("\nlegacy_open_seek_read_close_ns=")
    try out.writeUint64(oldNs)
    try out.writeAll("\nfs_read_range_ns=")
    try out.writeUint64(newNs)
    try out.writeAll("\n")
..
