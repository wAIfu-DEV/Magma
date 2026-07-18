mod main
work(value u64) !u64:
    ret value
..
main() !void:
    callback (u64) !u64 = work
    value u64 = try callback(7)
..
