mod main
check(flag bool) !void:
    if flag:
        throw "failure"
    ..
..
main() !void:
    try check(false)
..
