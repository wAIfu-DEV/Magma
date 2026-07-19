mod main
use "../errors.mg" errors
use "../strings.mg" strings

tracedFailure() !u8:
    throw errors.invalidArgument("traced failure")
..

pub main() !void:
    success := errors.ok()
    errors.printTrace(success)
    if errors.code(success) != 0 || errors.isOk(success) == false || errors.isError(success):
        throw errors.failure("ok error classification changed")
    ..
    failure := errors.invalidArgument("bad input")
    if errors.hasCode(failure, 2) == false || errors.isError(failure) == false || errors.is(failure, errors.invalidArgument("other")) == false || strings.compare(errors.message(failure), "bad input") == false:
        throw errors.failure("error behavior changed")
    ..
    if strings.compare(errors.toStr(failure), "invalid argument") == false:
        throw errors.failure("error category string changed")
    ..
    native := errors.native(123, "native")
    if errors.isNative(native) == false || errors.nativeCode(native) != 123:
        throw errors.failure("native error behavior changed")
    ..
    if errors.code(errors.failure("")) != 1 || errors.code(errors.outOfMemory("")) != 3 || errors.code(errors.endOfFile("")) != 4 || errors.code(errors.wouldOverflow("")) != 5 || errors.code(errors.invalidType("")) != 6 || errors.code(errors.outOfBounds("")) != 7:
        throw errors.failure("standard error codes changed")
    ..

    ignored u8, traced error = tracedFailure()
    trace := errors.trace(traced)
    if trace.isEmpty():
        throw errors.failure("throw did not record an error trace")
    ..
    if strings.compare(trace.function(), "tracedFailure") == false || strings.compare(trace.file(), "errors.mg") == false || trace.line() != 6 || trace.column() == 0 || trace.isTruncated():
        throw errors.failure("error trace metadata changed")
    ..
    parent := trace.next()
    if parent.isEmpty() == false:
        throw errors.failure("single throw recorded an unexpected parent frame")
    ..
..
