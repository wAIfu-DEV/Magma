mod main
identity(value u64) u64:
    ret value
..
callback (u64) u64 = identity
main() void:
    value u64 = callback(6)
    callback = identity
..
