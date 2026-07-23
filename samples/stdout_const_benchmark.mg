mod main

use "../std/io.mg"   io
use "../std/time.mg" time

const ITERATIONS u64 = 200000
const ROUNDS u64 = 8

runExisting() !u64:
    out := io.stdoutUnbuffered()
    start := time.ticks()
    i u64 = 0
    while i < ITERATIONS:
        try out.writeLn("magma stdout benchmark")
        i = i + 1
    ..
    ret time.elapsedUs(start)
..

runConstant() !u64:
    start := time.ticks()
    i u64 = 0
    while i < ITERATIONS:
        try io.printLn("magma stdout benchmark")
        i = i + 1
    ..
    ret time.elapsedUs(start)
..

pub main() !void:
    # Warm handle caches and generated code. Redirect stdout to NUL when running.
    existingWarmup := try runExisting()
    constantWarmup := try runConstant()

    existingTotal u64 = 0
    constantTotal u64 = 0
    round u64 = 0
    while round < ROUNDS:
        if round % 2 == 0:
            existingTotal = existingTotal + try runExisting()
            constantTotal = constantTotal + try runConstant()
        else:
            constantTotal = constantTotal + try runConstant()
            existingTotal = existingTotal + try runExisting()
        ..
        round = round + 1
    ..

    # Report after all measured stdout writes so output redirection can keep the
    # benchmark payload separate from the results.
    report := io.stderrUnbuffered()
    try report.writeAll("iterations=")
    try report.writeUint64(ITERATIONS)
    try report.writeAll(" rounds=")
    try report.writeUint64(ROUNDS)
    try report.writeAll("\nexisting warmup_us=")
    try report.writeUint64(existingWarmup)
    try report.writeAll(" average_us=")
    try report.writeUint64(existingTotal / ROUNDS)
    try report.writeAll("\nconstant warmup_us=")
    try report.writeUint64(constantWarmup)
    try report.writeAll(" average_us=")
    try report.writeUint64(constantTotal / ROUNDS)
    try report.writeAll("\n")
..
