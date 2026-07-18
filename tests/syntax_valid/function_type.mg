mod main
identity(value u64) u64:
    ret value
..
main() void:
    callback (u64) u64 = identity
    value u64 = callback(7)
    deferred (u64) u64
    deferred = identity
    second u64 = deferred(8)
..
