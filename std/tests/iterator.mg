mod main
use "std:errors" errors
use "std:iterator" iterator
use "std:cast" cast
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
