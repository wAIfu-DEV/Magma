mod main
# Unix benchmark: libc getenv versus the standard library's environ scan.

use "std:env" as env
use "std:errors" as errors
use "std:heap" as heap
use "std:io" as io
use "std:strings" as strings
use "std:time" as time

ext ext_getenv getenv(name u8*) u8*

legacyGetenv(name u8*) u8*:
    # SAFETY: getenv returns either a borrowed process-environment pointer or none.
    unsafe:
        @llvm("sideeffect", void)
        ret ext_getenv(name)
    ..
..

const ITERATIONS u64 = 100000

legacy(name u8*) u64:
    found u64 = 0
    for i u64 = 0 to ITERATIONS:
        if legacyGetenv(name) != none: found = found + 1 ..
    ..
    ret found
..

scanned() u64:
    found u64 = 0
    for i u64 = 0 to ITERATIONS:
        if env.has("PATH"): found = found + 1 ..
    ..
    ret found
..

pub main() !void:
    native := try strings.toCstr("PATH")
    defer heap.allocator().free(native)
    start := time.ticks()
    oldResult := legacy(native)
    oldNs := time.ticksToNs(time.elapsedTicks(start))
    start = time.ticks()
    newResult := scanned()
    newNs := time.ticksToNs(time.elapsedTicks(start))
    if oldResult != ITERATIONS || newResult != ITERATIONS:
        throw errors.failure("environment benchmark requires PATH")
    ..
    out := io.stdoutUnbuffered()
    try out.writeAll("environment lookups=")
    try out.writeUint64(ITERATIONS)
    try out.writeAll("\nlegacy_getenv_ns=")
    try out.writeUint64(oldNs)
    try out.writeAll("\nenviron_scan_ns=")
    try out.writeUint64(newNs)
    try out.writeAll("\n")
..
