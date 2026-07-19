mod main
use "../errors.mg" errors
use "../sort.mg" sort
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
    values[0] = 3
    values[1] = 1
    values[2] = 2
    sort.insertion[u64](values, compare)
    if values[0] != 1 || values[1] != 2 || values[2] != 3:
        throw errors.failure("sort behavior changed")
    ..
    sort.reverse[u64](values)
    if values[0] != 3 || values[2] != 1:
        throw errors.failure("sort reverse changed")
    ..
..
