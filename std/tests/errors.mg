mod main
use "../errors.mg" errors
use "../strings.mg" strings
pub main() !void:
    failure := errors.invalidArgument("bad input")
    if errors.hasCode(failure, 2) == false || strings.compare(errors.message(failure), "bad input") == false:
        throw errors.failure("error behavior changed")
    ..
..
