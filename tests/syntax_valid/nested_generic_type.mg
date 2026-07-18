mod main
Box[T](value T)
main() void:
    inner Box[u8] = Box[u8](value=1)
    outer Box[Box[u8]] = Box[Box[u8]](value=inner)
..
