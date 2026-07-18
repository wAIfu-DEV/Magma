mod main
countdown(value u64) u64:
    if value == 0:
        ret 0
    ..
    ret countdown(value - 1)
..
main() void:
    value u64 = countdown(2)
..
