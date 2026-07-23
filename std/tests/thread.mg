mod main

use "std:errors" errors
use "std:thread" thread

worker(context ptr) u64:
    value u64* = context
    *value = 42
    ret 0
..

pub main() !void:
    value u64 = 0
    t := try thread.new[ptr](worker, addrof value)
    finished bool = try t.isFinished()
    while finished == false:
        thread.yield()
        finished = try t.isFinished()
    ..
    try t.join()

    if value != 42:
        throw errors.failure("thread did not publish its result before join")
    ..

    thread.yield()

    valueA u64 = 0
    valueB u64 = 0
    threads := array thread.Thread[2]
    threads[0] = try thread.new[ptr](worker, addrof valueA)
    threads[1] = try thread.new[ptr](worker, addrof valueB)
    try thread.joinAll(threads)
    if valueA != 42 || valueB != 42:
        throw errors.failure("joinAll returned before every thread finished")
    ..
..
