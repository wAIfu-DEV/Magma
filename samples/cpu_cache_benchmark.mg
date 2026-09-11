mod main
# Linux benchmark: repeated sysconf calls versus std:cpu's process cache.

use "std:c" as c
use "std:cast" as cast
use "std:cpu" as cpu
use "std:errors" as errors
use "std:io" as io
use "std:time" as time

ext ext_sysconf sysconf(name c.int) c.long

const ITERATIONS u64 = 100000

legacy() u64:
    total u64 = 0
    for i u64 = 0 to ITERATIONS:
        count i64 = ext_sysconf(84)
        if count > 0: total = total + cast.itou(count) ..
    ..
    ret total
..

cached() u64:
    total u64 = 0
    for i u64 = 0 to ITERATIONS:
        total = total + cpu.coreCount()
    ..
    ret total
..

pub main() !void:
    expected := cpu.coreCount() * ITERATIONS
    start := time.ticks()
    oldResult := legacy()
    oldNs := time.ticksToNs(time.elapsedTicks(start))
    start = time.ticks()
    newResult := cached()
    newNs := time.ticksToNs(time.elapsedTicks(start))
    if oldResult != expected || newResult != expected:
        throw errors.failure("processor-count benchmark result changed")
    ..
    out := io.stdoutUnbuffered()
    try out.writeAll("processor count calls=")
    try out.writeUint64(ITERATIONS)
    try out.writeAll("\nlegacy_sysconf_ns=")
    try out.writeUint64(oldNs)
    try out.writeAll("\nprocess_cache_ns=")
    try out.writeUint64(newNs)
    try out.writeAll("\n")
..
