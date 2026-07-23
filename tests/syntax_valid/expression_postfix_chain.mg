mod main
Box(value u64)
Box.get() u64:
    ret this.value
..
main() void:
    boxes := array Box[1]
    value u64 = boxes[0].get()
..
