mod main
identity[T](value T) T:
    ret value
..
main() void:
    callback := identity[u64]
    value u64 = callback(9)
..
