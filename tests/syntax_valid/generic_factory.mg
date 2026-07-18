mod main
Box[T](value T)
boxOf[T](value T) Box[T]:
    ret Box[T](value=value)
..
main() void:
    box Box[u64] = boxOf[u64](2)
..
