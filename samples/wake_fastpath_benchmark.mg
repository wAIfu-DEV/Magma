mod main
# POSIX retained-token benchmark: condition-variable calls versus atomics.

use "std:c" as c
use "std:errors" as errors
use "std:io" as io
use "std:time" as time
use "std:wake" as wake

link "pthread"

Opaque(part0 u128, part1 u128, part2 u128, part3 u128, part4 u128, part5 u128, part6 u128, part7 u128)
LegacyWake(lock Opaque, condition Opaque, count u64)

ext ext_mutex_init pthread_mutex_init(value ptr, attributes ptr) c.int
ext ext_mutex_lock pthread_mutex_lock(value ptr) c.int
ext ext_mutex_unlock pthread_mutex_unlock(value ptr) c.int
ext ext_mutex_destroy pthread_mutex_destroy(value ptr) c.int
ext ext_cond_init pthread_cond_init(value ptr, attributes ptr) c.int
ext ext_cond_signal pthread_cond_signal(value ptr) c.int
ext ext_cond_destroy pthread_cond_destroy(value ptr) c.int

const ITERATIONS u64 = 1000000

legacy() !void:
    value LegacyWake
    value.count = 0
    if ext_mutex_init(addrof value.lock, none) != 0: throw errors.failure("wake mutex init failed") ..
    if ext_cond_init(addrof value.condition, none) != 0: throw errors.failure("wake condition init failed") ..
    for i u64 = 0 to ITERATIONS:
        ext_mutex_lock(addrof value.lock)
        value.count = value.count + 1
        ext_cond_signal(addrof value.condition)
        ext_mutex_unlock(addrof value.lock)
        ext_mutex_lock(addrof value.lock)
        value.count = value.count - 1
        ext_mutex_unlock(addrof value.lock)
    ..
    ext_cond_destroy(addrof value.condition)
    ext_mutex_destroy(addrof value.lock)
..

atomicTokens() !void:
    value := try wake.new(wake.condition())
    for i u64 = 0 to ITERATIONS:
        try value.notify()
        try value.wait()
    ..
    try value.free()
..

pub main() !void:
    start := time.ticks()
    try legacy()
    oldNs := time.ticksToNs(time.elapsedTicks(start))
    start = time.ticks()
    try atomicTokens()
    newNs := time.ticksToNs(time.elapsedTicks(start))
    out := io.stdoutUnbuffered()
    try out.writeAll("retained wake tokens=")
    try out.writeUint64(ITERATIONS)
    try out.writeAll("\nlegacy_condition_ns=")
    try out.writeUint64(oldNs)
    try out.writeAll("\natomic_token_ns=")
    try out.writeUint64(newNs)
    try out.writeAll("\n")
..
