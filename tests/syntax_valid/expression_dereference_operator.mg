mod main
main() void:
    value u64 = 1
    pointer u64* = addrof value
    copy u64 = *pointer
..
