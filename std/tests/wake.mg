mod main

use "../errors.mg" errors
use "../wake.mg" wake
use "../footgun.mg" footgun

check(strategy u8) !void:
    signal := try wake.new(strategy)
    try signal.notify()
    try signal.wait()
    try signal.free()
..

pub main() !void:
    if wake.condition() == wake.semaphore():
        throw errors.failure("wake strategies are not distinct")
    ..
    try check(wake.condition())
    try check(wake.semaphore())

    invalid wake.Wake, invalidErr error = wake.new(255)
    if invalidErr.ok():
        footgun.drop[wake.Wake](invalid)
        throw errors.failure("wake accepted an invalid strategy")
    ..
    if errors.code(invalidErr) != 2:
        throw errors.failure("wake returned the wrong invalid-strategy error")
    ..
..
