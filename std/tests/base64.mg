mod main
use "std:base64" base64
use "std:errors" errors
use "std:heap" heap
use "std:slices" slices
use "std:strings" strings

pub main() !void:
    a := heap.allocator()
    data := array u8[6]
    data[0] = 102
    data[1] = 111
    data[2] = 111
    data[3] = 98
    data[4] = 97
    data[5] = 114
    view u8[] = slices.fromPtr(slices.toPtr(data), 6)
    encoded := try base64.encode(view)
    defer encoded.free(a)
    if strings.compare(encoded, "Zm9vYmFy") == false:
        throw errors.failure("Base64 encoding changed")
    ..
    decoded := try base64.decode("Zm8=")
    defer slices.free(decoded)
    if slices.count(decoded) != 2 || decoded[0] != 102 || decoded[1] != 111:
        throw errors.failure("Base64 decoding changed")
    ..
    bad, badError := base64.decode("Zh==")
    if badError.ok():
        slices.free(bad)
        throw errors.failure("non-canonical Base64 accepted")
    ..
..
