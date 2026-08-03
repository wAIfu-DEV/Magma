mod main

cleanup(value u64) void: ..

work() !u64:
    value u64 = 1
    onerror cleanup(value)
    onerror:
        cleanup(value)
    ..
    ret value
..
