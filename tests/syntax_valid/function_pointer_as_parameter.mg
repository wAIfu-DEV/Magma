mod main
identity(value u64) u64:
    ret value
..
apply(callback (u64) u64, value u64) u64:
    ret callback(value)
..
main() void:
    result u64 = apply(identity, 5)
..
