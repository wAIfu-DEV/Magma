mod main
Counter(value u64)
Counter.increment() void: this.value = this.value + 1 ..
pub main() void:
    counter Counter
    pointer Counter* = addrof counter
    pointer.increment()
..
