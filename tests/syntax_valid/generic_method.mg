mod main
Box[T](value T)
Box[T].get() T:
    ret this.value
..
main() void:
    box Box[u64] = Box[u64](value=3)
    value u64 = box.get()
..
