mod main
combine(
    left u64,
    right u64,
) u64:
    ret left + right
..
main() void:
    value u64 = combine(1, 2)
..
