mod main
use "../errors.mg" errors
use "../random.mg" random
pub main() !void:
    first := random.new(123)
    second := random.new(123)
    if first.next() != second.next() || first.bounded(10) >= 10 || first.boolean() != second.boolean():
        throw errors.failure("random behavior changed")
    ..
..
