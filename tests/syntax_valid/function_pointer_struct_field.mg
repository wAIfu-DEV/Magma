mod main
Handler(callback (u64) u64)
identity(value u64) u64:
    ret value
..
main() void:
    handler Handler = Handler(callback=identity)
    value u64 = handler.callback(4)
..
