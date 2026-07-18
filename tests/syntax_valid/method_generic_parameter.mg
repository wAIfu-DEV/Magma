mod main
Box[T](value T)
Box[T].choose[U](other U) U:
    ret other
..
main() void:
    box Box[u64] = Box[u64](value=3)
    value u8 = box.choose[u8](4)
..
