mod main
use "../errors.mg" errors
use "../strings.mg" strings

tracedFailure() !u8:
    throw errors.invalidArgument("traced failure")
..

pub main() !void:
    failure := errors.invalidArgument("bad input")
    if errors.hasCode(failure, 2) == false || strings.compare(errors.message(failure), "bad input") == false:
        throw errors.failure("error behavior changed")
    ..

    ignored u8, traced error = tracedFailure()
    trace := errors.trace(traced)
    if trace.isEmpty():
        throw errors.failure("throw did not record an error trace")
    ..
    if strings.compare(trace.function(), "tracedFailure") == false || trace.line() != 6:
        throw errors.failure("error trace metadata changed")
    ..
    parent := trace.next()
    if parent.isEmpty() == false:
        throw errors.failure("single throw recorded an unexpected parent frame")
    ..
..
