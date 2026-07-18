mod main
sum(
    left u64
    right u64
) u64:
    ret left + right
..
main() void:
    value u64 = sum(1, 2)
..
