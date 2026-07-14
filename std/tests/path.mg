mod main
use "../errors.mg" errors
use "../path.mg" path
use "../strings.mg" strings
pub main() !void:
    if strings.compare(path.base("one/two.txt"), "two.txt") == false:
        throw errors.failure("path base changed")
    ..
    if strings.compare(path.extension("one/two.txt"), ".txt") == false:
        throw errors.failure("path extension changed")
    ..
..
