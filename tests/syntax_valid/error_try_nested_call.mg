mod main
produce() !u64:
    ret 4
..
consume(value u64) u64:
    ret value
..
main() !void:
    value u64 = consume(try produce())
..
