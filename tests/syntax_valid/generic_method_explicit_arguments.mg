mod main
Box[T](value T)
Box[T].replace[U](value U) Box[U]:
    ret Box[U](value=value)
..
main() void:
    box Box[u64] = Box[u64](value=1)
    other Box[bool] = box.replace[bool](true)
..
