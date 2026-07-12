mod strconv

use "allocator.mg" alc
use "strings.mg" strings
use "errors.mg" errors
use "cast.mg" cast

pub parseUint(s str) !u64:
    n := strings.countBytes(s)
    if n == 0:
        throw errors.invalidArgument("empty unsigned integer")
    ..
    value u64 = 0
    i u64 = 0
    while i < n:
        c := strings.byteAt(s, i)
        if c < 48 || c > 57:
            throw errors.invalidArgument("invalid unsigned integer")
        ..
        digit := cast.u8to64(c - 48)
        if value > 1844674407370955161 || (value == 1844674407370955161 && digit > 5):
            throw errors.wouldOverflow("unsigned integer overflow")
        ..
        value = value * 10 + digit
        i = i + 1
    ..
    ret value
..

pub parseBool(s str) !bool:
    if strings.compare(s, "true"):
        ret true
    elif strings.compare(s, "false"):
        ret false
    ..
    throw errors.invalidArgument("invalid boolean")
..

pub formatUint(a alc.Allocator, value u64) !$str:
    remaining := value
    digits u64 = 1
    tmp := value
    while tmp >= 10:
        tmp = tmp / 10
        digits = digits + 1
    ..
    out u8* = try a.alloc(digits)
    i := digits
    while i > 0:
        i = i - 1
        out[i] = cast.u64to8((remaining % 10) + 48)
        remaining = remaining / 10
    ..
    ret strings.fromPtrNoCopy(out, digits)
..
