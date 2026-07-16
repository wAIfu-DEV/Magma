mod main
use "../errors.mg" errors
use "../iterator.mg" iterator
use "../cast.mg" cast
hasData(impl ptr, index u64*) bool:
    ret *index < 2
..
next(impl ptr, index u64*) !u64:
    value := *index + 10
    *index = *index + 1
    ret value
..
pub main() !void:
    values := iterator.new[u64](none, hasData, next)
    first := try values.next()
    if first != 10 || values.hasData() == false:
        throw errors.failure("iterator behavior changed")
    ..
..
