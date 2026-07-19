mod main
use "../errors.mg" errors
use "../hash.mg" hash
pub main() !void:
    value u8[3]
    value[0] = 97
    value[1] = 98
    value[2] = 99
    if hash.string("abc") != hash.string("abc") || hash.bytes(value) != hash.string("abc"):
        throw errors.failure("hash is not deterministic")
    ..
..
