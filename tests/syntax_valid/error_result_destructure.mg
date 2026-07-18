mod main
value() !u64:
    ret 1
..
main() void:
    result u64, err error = value()
    inferred, inferredErr := value()
..
