mod main
identity[T](value T) T:
    ret value
..
main() void:
    value u64 = identity[](1)
..
