mod main

use "../cpu.mg" cpu
use "../errors.mg" errors

pub main() !void:
    if cpu.coreCount() == 0:
        throw errors.failure("CPU core count must be greater than zero")
    ..
..
