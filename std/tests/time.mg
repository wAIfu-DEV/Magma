mod main
use "../errors.mg" errors
use "../time.mg" time
pub main() !void:
    start := time.ticks()
    if time.ticksToMs(time.msToTicks(5)) != 5 || time.elapsedTicks(start) > time.ticks():
        throw errors.failure("time conversion changed")
    ..
..
