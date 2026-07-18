mod main
produce() !u64:
    ret 1
..
main() void:
    value u64, err error = produce()
..
