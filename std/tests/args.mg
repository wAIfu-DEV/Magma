mod main
use "std:args" args
use "std:errors" errors
use "std:strings" strings

pub main() !void:
    raw := array str[3]
    raw[0] = "tool"
    raw[1] = "first"
    raw[2] = ""
    parsed := args.new(raw)
    if parsed.count() != 2 || strings.compare(try parsed.executable(), "tool") == false:
        throw errors.failure("args header changed")
    ..
    if strings.compare(try parsed.get(0), "first") == false || (try parsed.get(1)).countBytes() != 0:
        throw errors.failure("args values changed")
    ..
..
