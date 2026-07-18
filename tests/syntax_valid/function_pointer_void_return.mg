mod main
consume(value u64) void:
..
main() void:
    callback (u64) void = consume
    callback(9)
..
