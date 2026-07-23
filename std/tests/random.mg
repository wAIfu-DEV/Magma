mod main
use "std:errors" errors
use "std:random" random
pub main() !void:
    first := random.new(123)
    second := random.new(123)
    if first.next() != second.next() || first.bounded(10) >= 10 || first.bool() != second.bool():
        throw errors.failure("random behavior changed")
    ..
..
