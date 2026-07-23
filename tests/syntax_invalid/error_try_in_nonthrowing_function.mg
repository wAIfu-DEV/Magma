mod main

fail() !void:
    throw "failure"
..

pub main() void:
    try fail()
..
