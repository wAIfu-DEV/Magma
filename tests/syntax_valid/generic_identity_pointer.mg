mod main
identityPointer[T](value T*) T*:
    ret value
..
main() void:
    pointer u64*
    result u64* = identityPointer[u64](pointer)
..
