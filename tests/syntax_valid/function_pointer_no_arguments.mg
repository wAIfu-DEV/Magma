mod main
answer() u64:
    ret 42
..
main() void:
    callback () u64 = answer
    value u64 = callback()
..
