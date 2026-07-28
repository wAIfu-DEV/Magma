mod main

Inner(value u64)
Outer(inner Inner*)

main() void:
    inner Inner
    outer Outer = Outer(inner=addrof inner)
    outer.inner.value = 42
    value u64 = outer.inner.value
..
