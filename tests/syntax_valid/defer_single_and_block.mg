mod main
cleanup() void:
..
main() void:
    defer cleanup()
    defer:
        cleanup()
        cleanup()
    ..
..
