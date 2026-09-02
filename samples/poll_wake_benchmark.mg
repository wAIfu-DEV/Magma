mod main
# Linux benchmark: one eventfd write per interrupt versus wake coalescing.

use "std:c" c
use "std:errors" errors
use "std:heap" heap
use "std:io" io
use "std:net/poll" poll
use "std:time" time

ext ext_eventfd eventfd(initialValue c.unsigned_int, flags c.int) c.int
ext ext_write write(fd c.int, buffer ptr, count u64) i64
ext ext_read read(fd c.int, buffer ptr, count u64) i64
ext ext_close close(fd c.int) c.int

const INTERRUPTS u64 = 100000

legacy() !void:
    fd := ext_eventfd(0, 0x800 | 0x80000)
    if fd < 0: throw errors.failure("eventfd failed") ..
    value u64 = 1
    for i u64 = 0 to INTERRUPTS:
        if ext_write(fd, addrof value, sizeof u64) < 0:
            ext_close(fd)
            throw errors.failure("eventfd write failed")
        ..
    ..
    ext_read(fd, addrof value, sizeof u64)
    ext_close(fd)
..

coalesced() !void:
    value := try poll.new(heap.allocator(), 1)
    for i u64 = 0 to INTERRUPTS:
        try value.interrupt()
    ..
    events := array poll.Event[1]
    try value.wait(events, 0)
    try value.close()
..

pub main() !void:
    start := time.ticks()
    try legacy()
    oldNs := time.ticksToNs(time.elapsedTicks(start))
    start = time.ticks()
    try coalesced()
    newNs := time.ticksToNs(time.elapsedTicks(start))
    out := io.stdoutUnbuffered()
    try out.writeAll("interrupts=")
    try out.writeUint64(INTERRUPTS)
    try out.writeAll("\nlegacy_eventfd_ns=")
    try out.writeUint64(oldNs)
    try out.writeAll("\ncoalesced_eventfd_ns=")
    try out.writeUint64(newNs)
    try out.writeAll("\n")
..
