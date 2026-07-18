mod main
pub exported() u64:
    ret 1
..
main() void:
    value u64 = exported()
..
