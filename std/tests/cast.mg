mod main
use "../cast.mg" cast
use "../errors.mg" errors
pub main() !void:
    if cast.u64to8(258) != 2 || cast.i64to32(-7) != -7 || cast.ftou(4.0) != 4:
        throw errors.failure("cast behavior changed")
    ..
..
