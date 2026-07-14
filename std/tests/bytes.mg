mod main
use "../bytes.mg" bytes
use "../errors.mg" errors
pub main() !void:
    value u8[3]
    value[0] = 1
    value[1] = 2
    value[2] = 3
    index := try bytes.indexByte(value, 2)
    if index != 1 || bytes.contains(value, 3) == false:
        throw errors.failure("bytes search changed")
    ..
    bytes.reverse(value)
    if value[0] != 3 || value[2] != 1:
        throw errors.failure("bytes reverse changed")
    ..
..
