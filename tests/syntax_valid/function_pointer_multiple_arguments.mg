mod main
add(left u64, right u64) u64:
    ret left + right
..
main() void:
    callback (u64, u64) u64 = add
    value u64 = callback(2, 3)
..
