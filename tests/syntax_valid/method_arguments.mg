mod main
Counter(value u64)
Counter.add(amount u64) u64:
    this.value = this.value + amount
    ret this.value
..
main() void:
    counter Counter
    value u64 = counter.add(3)
..
