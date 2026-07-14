mod main
use "../errors.mg" errors
use "../hash.mg" hash
pub main() !void:
    if hash.string("abc") != hash.string("abc"):
        throw errors.failure("hash is not deterministic")
    ..
..
