mod main
identity(value u64) u64:
    ret value
..
select() (u64) u64:
    ret identity
..
main() void:
    callback (u64) u64 = select()
..
