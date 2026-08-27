mod main

use "std:sha1" sha1
use "std:errors" errors
use "std:slices" slices
use "std:strings" strings

hexDigit(value u8) u8:
    if value < 10:
        ret 48 + value
    ..
    ret 97 + value - 10
..

pub main() !void:
    digest := array u8[20]
    input u8[] = slices.fromPtr(strings.toPtr("abc"), 3)
    output u8[] = digest
    try sha1.sum(input, output)
    encoded := array u8[40]
    for i u64 = 0 to 20:
        bounded i < 20, i * 2 < 40, i * 2 + 1 < 40:
            encoded[i * 2] = hexDigit(digest[i] >> 4)
            encoded[i * 2 + 1] = hexDigit(digest[i] & 15)
        ..
    ..
    actual := strings.fromPtrNoCopy(slices.toPtr(encoded), 40)
    if strings.compare(actual, "a9993e364706816aba3e25717850c26c9cd0d89d") == false:
        throw errors.failure("SHA-1 digest changed")
    ..
..
