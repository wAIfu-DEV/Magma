mod main
use "std:errors" errors
use "std:heap" heap
use "std:hex" hex
use "std:slices" slices
use "std:strings" strings

pub main() !void:
    a := heap.allocator()
    input := array u8[3]
    input[0] = 0
    input[1] = 0xAB
    input[2] = 0xFF
    view u8[] = slices.fromPtr(slices.toPtr(input), 3)
    encoded := try hex.encode(view)
    defer encoded.free(a)
    if strings.compare(encoded, "00abff") == false:
        throw errors.failure("hexadecimal encoding changed")
    ..
    decoded := try hex.decode("00ABff")
    defer slices.free(decoded)
    if slices.count(decoded) != 3 || decoded[1] != 0xAB || decoded[2] != 0xFF:
        throw errors.failure("hexadecimal decoding changed")
    ..
    bad, badError := hex.decode("abc")
    if badError.ok():
        slices.free(bad)
        throw errors.failure("odd hexadecimal input accepted")
    ..
..
