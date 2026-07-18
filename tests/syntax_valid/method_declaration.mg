mod main
Counter(value u64)
Counter.get() u64:
    ret this.value
..
main() void:
    counter Counter
    value u64 = counter.get()
..
