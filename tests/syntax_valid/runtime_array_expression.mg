mod main

Pair[T, U](first T, second U)

make(count u64) void:
    bytes := array u8[count]
    pairs := array Pair[u8, u64][count + 1]
    bytes[0] = 7
    pairs[0] = Pair[u8, u64](first=1, second=2)
..

main() void:
    make(3)
..
