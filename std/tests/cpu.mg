mod main

use "std:cpu" cpu
use "std:errors" errors

pub main() !void:
    if cpu.coreCount() == 0:
        throw errors.failure("CPU core count must be greater than zero")
    ..
..
