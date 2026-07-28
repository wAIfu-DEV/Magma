mod main
touch(value u64*) void: *value = *value + 1 ..
pub main() void:
    value u64 = 0
    if true:
        defer touch(addrof value)
    ..
..
