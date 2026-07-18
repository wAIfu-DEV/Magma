mod main
choose(flag bool) u64:
    if flag:
        ret 1
    ..
    ret 2
..
main() void:
    value u64 = choose(true)
..
