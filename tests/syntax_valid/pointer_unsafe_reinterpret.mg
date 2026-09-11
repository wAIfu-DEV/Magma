mod main

main() void:
    value u64 = 0
    source u64* = addrof value
    unsafe:
        target u8* = source
    ..
..
