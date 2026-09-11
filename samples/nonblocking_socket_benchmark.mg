mod main
# Linux benchmark: socket+fcntl versus atomic nonblocking socket creation.

use "std:c" as c
use "std:errors" as errors
use "std:io" as io
use "std:net/address" as address
use "std:net/socket" as socket
use "std:time" as time

ext ext_socket socket(domain c.int, kind c.int, protocol c.int) c.int
ext ext_fcntl fcntl(fd c.int, command c.int, value c.int) c.int
ext ext_close close(fd c.int) c.int

const ITERATIONS u64 = 1000

legacy() !void:
    for i u64 = 0 to ITERATIONS:
        fd := ext_socket(2, 1, 0)
        if fd < 0: throw errors.failure("legacy socket failed") ..
        flags := ext_fcntl(fd, 3, 0)
        if flags < 0 || ext_fcntl(fd, 4, flags | 2048) < 0:
            ext_close(fd)
            throw errors.failure("legacy fcntl failed")
        ..
        if ext_close(fd) != 0: throw errors.failure("legacy close failed") ..
    ..
..

combined() !void:
    for i u64 = 0 to ITERATIONS:
        value := try socket.openNonBlocking(address.FAMILY_IPV4, socket.TYPE_STREAM)
        try value.close()
    ..
..

pub main() !void:
    start := time.ticks()
    try legacy()
    oldNs := time.ticksToNs(time.elapsedTicks(start))
    start = time.ticks()
    try combined()
    newNs := time.ticksToNs(time.elapsedTicks(start))
    out := io.stdoutUnbuffered()
    try out.writeAll("nonblocking socket lifetimes=")
    try out.writeUint64(ITERATIONS)
    try out.writeAll("\nlegacy_socket_fcntl_ns=")
    try out.writeUint64(oldNs)
    try out.writeAll("\ncombined_flags_ns=")
    try out.writeUint64(newNs)
    try out.writeAll("\n")
..
