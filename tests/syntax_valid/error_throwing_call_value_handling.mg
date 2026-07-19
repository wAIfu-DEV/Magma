mod main

produce() !u64:
    ret 42
..

consume(value u64) void:
..

main() !void:
    # Ignoring a throwing call's return value is permitted.
    produce()

    # Consumed values must be explicitly unwrapped.
    value := try produce()
    consume(try produce())

    # Destructuring handles both outcomes without try.
    first, firstErr := produce()
..
