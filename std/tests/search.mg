mod main
use "../errors.mg" errors
use "../search.mg" search
compare(a u64, b u64) i64:
    if a < b:
        ret -1
    elif a > b:
        ret 1
    ..
    ret 0
..
pub main() !void:
    values u64[3]
    values[0] = 2
    values[1] = 4
    values[2] = 6
    linear := try search.linear[u64](values, 4, compare)
    binary := try search.binary[u64](values, 6, compare)
    if linear != 1 || binary != 2:
        throw errors.failure("search behavior changed")
    ..
..
