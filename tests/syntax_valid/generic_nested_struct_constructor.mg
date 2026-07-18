mod main
Box[T](value T)
main() void:
    inner Box[u64] = Box[u64](value=1)
    outer Box[Box[u64]] = Box[Box[u64]](value=inner)
..
