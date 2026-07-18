mod main
identity(value u64) u64:
    ret value
..
main() void:
    callback := identity
    value u64 = callback(10)
..
