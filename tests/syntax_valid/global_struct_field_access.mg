mod main

Inner(value u64)
Outer(inner Inner*, direct Inner)

globalOuter Outer

main() void:
    globalOuter.direct.value = 7
    directValue u64 = globalOuter.direct.value

    inner Inner
    globalOuter.inner = addrof inner
    globalOuter.inner.value = 42
    pointerValue u64 = globalOuter.inner.value
    fieldPointer u64* = addrof globalOuter.direct.value
..
