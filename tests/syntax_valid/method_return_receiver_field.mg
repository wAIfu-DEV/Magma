mod main
Box(value u64)
Box.get() u64:
    ret this.value
..
main() void:
    box Box = Box(value=7)
    value u64 = box.get()
..
