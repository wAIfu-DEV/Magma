mod main
pass(err error) !u64:
    throw err
    ret 1
..
forward(err error) !u64:
    ret try pass(err)
..
main() void:
..
