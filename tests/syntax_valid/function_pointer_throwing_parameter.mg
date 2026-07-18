mod main
work(value u64) !u64:
    ret value
..
apply(callback (u64) !u64, value u64) !u64:
    ret try callback(value)
..
main() !void:
    value u64 = try apply(work, 8)
..
