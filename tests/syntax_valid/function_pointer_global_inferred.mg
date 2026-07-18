mod main
identity(value u64) u64:
    ret value
..
callback := identity
main() void:
    value u64 = callback(5)
..
