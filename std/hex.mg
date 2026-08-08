mod hex
# Strict hexadecimal encoding and decoding.

use "std:allocator" alc
use "std:checked" checked
use "std:errors" errors
use "std:slices" slices
use "std:strings" strings

pub encodedSize(inputSize u64) !u64:
    ret try checked.uMul(inputSize, 2)
..

pub decodedSize(text str) !u64:
    if (text.countBytes() & 1) != 0:
        throw errors.invalidArgument("hexadecimal input has odd length")
    ..
    ret text.countBytes() / 2
..

digit(nibble u8, uppercase bool) u8:
    if nibble < 10:
        ret 48 + nibble
    ..
    if uppercase:
        ret 65 + nibble - 10
    ..
    ret 97 + nibble - 10
..

decodeNibble(byte u8) !u8:
    if byte >= 48 && byte <= 57:
        ret byte - 48
    ..
    if byte >= 65 && byte <= 70:
        ret byte - 65 + 10
    ..
    if byte >= 97 && byte <= 102:
        ret byte - 97 + 10
    ..
    throw errors.invalidArgument("invalid hexadecimal digit")
..

encodeToCase(input u8[], output u8[], uppercase bool) !u64:
    needed := try encodedSize(slices.count(input))
    if slices.count(output) < needed:
        throw errors.invalidArgument("hexadecimal output buffer is too small")
    ..
    for i u64 = 0 to slices.count(input):
        output[i * 2] = digit(input[i] >> 4, uppercase)
        output[i * 2 + 1] = digit(input[i] & 0x0F, uppercase)
    ..
    ret needed
..

pub encodeTo(input u8[], output u8[]) !u64:
    ret try encodeToCase(input, output, false)
..

pub encodeUpperTo(input u8[], output u8[]) !u64:
    ret try encodeToCase(input, output, true)
..

pub decodeTo(text str, output u8[]) !u64:
    needed := try decodedSize(text)
    if slices.count(output) < needed:
        throw errors.invalidArgument("hexadecimal output buffer is too small")
    ..
    input := strings.toPtr(text)
    for i u64 = 0 to needed:
        high := try decodeNibble(input[i * 2])
        low := try decodeNibble(input[i * 2 + 1])
        output[i] = (high << 4) | low
    ..
    ret needed
..

encodeAllocated(a alc.Allocator, input u8[], uppercase bool) !$str:
    size := try encodedSize(slices.count(input))
    result $str = try strings.alloc(a, size)
    onerror result.free(a)
    output u8[] = slices.fromPtr(strings.toPtr(result), size)
    try encodeToCase(input, output, uppercase)
    ret result
..

pub encode(a alc.Allocator, input u8[]) !$str:
    ret try encodeAllocated(a, input, false)
..

pub encodeUpper(a alc.Allocator, input u8[]) !$str:
    ret try encodeAllocated(a, input, true)
..

pub decode(a alc.Allocator, text str) !$u8[]:
    size := try decodedSize(text)
    if size == 0:
        ret slices.fromPtr(none, 0)
    ..
    result := try slices.alloc[u8](a, size)
    onerror slices.free(a, result)
    try decodeTo(text, result)
    ret result
..
