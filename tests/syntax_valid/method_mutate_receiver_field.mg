mod main
Counter(value u64)
Counter.increment() void:
    this.value = this.value + 1
..
main() void:
    counter Counter
    counter.increment()
..
