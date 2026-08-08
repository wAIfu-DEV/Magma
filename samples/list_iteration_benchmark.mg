mod main

use "std:allocator" allocator
use "std:errors" errors
use "std:heap" heap
use "std:io" io
use "std:list" list
use "std:time" time

const ITEMS u64 = 1000000
const ROUNDS u64 = 20

# Keep the three traversal forms in separate functions so their optimized LLVM
# is easy to locate when this sample is compiled with --emit llvm -O 3.
sumGet(values list.List[u64]*) !u64:
    total u64 = 0
    i u64 = 0
    loop i < values.count():
        total = total + try values.get(i)
        i = i + 1
    ..
    ret total
..

sumView(values list.List[u64]*) u64:
    items := values.view()
    total u64 = 0
    i u64 = 0
    loop i < items.count():
        total = total + items[i]
        i = i + 1
    ..
    ret total
..

sumIterator(values list.List[u64]*) !u64:
    iterator := values.iterator()
    total u64 = 0
    loop iterator.hasData():
        total = total + try iterator.next()
    ..
    ret total
..

check(actual u64, expected u64) !void:
    if actual != expected:
        throw errors.failure("list iteration benchmark checksum mismatch")
    ..
..

pub main() !void:
    a allocator.Allocator = heap.allocator()
    values := try list.new[u64](a, none)
    defer values.free()

    # A runtime-dependent payload prevents LLVM from deriving a constant sum.
    seed u64 = time.ticks()
    expected u64 = 0
    i u64 = 0
    loop i < ITEMS:
        value u64 = i ^ seed
        try values.pushRight(value)
        expected = expected + value
        i = i + 1
    ..

    # Warm all paths before timing and validate their observable results.
    try check(try sumGet(addrof values), expected)
    try check(sumView(addrof values), expected)
    try check(try sumIterator(addrof values), expected)

    getUs u64 = 0
    viewUs u64 = 0
    iteratorUs u64 = 0
    round u64 = 0
    loop round < ROUNDS:
        # Rotate order to reduce systematic temperature and scheduling bias.
        start u64 = 0
        result u64 = 0
        if round % 3 == 0:
            start = time.ticks()
            result = try sumGet(addrof values)
            getUs = getUs + time.elapsedUs(start)
            try check(result, expected)

            start = time.ticks()
            result = sumView(addrof values)
            viewUs = viewUs + time.elapsedUs(start)
            try check(result, expected)

            start = time.ticks()
            result = try sumIterator(addrof values)
            iteratorUs = iteratorUs + time.elapsedUs(start)
            try check(result, expected)
        elif round % 3 == 1:
            start = time.ticks()
            result = sumView(addrof values)
            viewUs = viewUs + time.elapsedUs(start)
            try check(result, expected)

            start = time.ticks()
            result = try sumIterator(addrof values)
            iteratorUs = iteratorUs + time.elapsedUs(start)
            try check(result, expected)

            start = time.ticks()
            result = try sumGet(addrof values)
            getUs = getUs + time.elapsedUs(start)
            try check(result, expected)
        else:
            start = time.ticks()
            result = try sumIterator(addrof values)
            iteratorUs = iteratorUs + time.elapsedUs(start)
            try check(result, expected)

            start = time.ticks()
            result = try sumGet(addrof values)
            getUs = getUs + time.elapsedUs(start)
            try check(result, expected)

            start = time.ticks()
            result = sumView(addrof values)
            viewUs = viewUs + time.elapsedUs(start)
            try check(result, expected)
        ..
        round = round + 1
    ..

    out := io.stdoutUnbuffered()
    try out.writeAll("List iteration benchmark (-O3)\nitems=")
    try out.writeUint64(ITEMS)
    try out.writeAll(" rounds=")
    try out.writeUint64(ROUNDS)
    try out.writeAll(" checksum=")
    try out.writeUint64(expected)
    try out.writeAll("\nget average_us=")
    try out.writeUint64(getUs / ROUNDS)
    try out.writeAll("\ncached view average_us=")
    try out.writeUint64(viewUs / ROUNDS)
    try out.writeAll("\niterator average_us=")
    try out.writeUint64(iteratorUs / ROUNDS)
    try out.writeAll("\nfastest=")
    if getUs < viewUs && getUs < iteratorUs:
        try out.writeAll("get")
    elif viewUs < getUs && viewUs < iteratorUs:
        try out.writeAll("cached view")
    elif iteratorUs < getUs && iteratorUs < viewUs:
        try out.writeAll("iterator")
    else:
        try out.writeAll("tie")
    ..
    try out.writeAll("\n")
..
