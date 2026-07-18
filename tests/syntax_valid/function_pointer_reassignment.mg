mod main
first(value u64) u64:
    ret value
..
second(value u64) u64:
    ret value + 1
..
main() void:
    callback (u64) u64 = first
    callback = second
    value u64 = callback(3)
..
