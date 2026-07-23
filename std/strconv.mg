mod strconv
# Parses numbers and formats primitive values as owned strings.

use "std:allocator" alc
use "std:strings" strings
use "std:errors" errors
use "std:cast" cast

# Parses a non-empty decimal string as u64.
# @complexity O(N)
# @throws invalidArgument for non-digits, empty input, or overflow
# @example
#   value := try strconv.parseUint("184")
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

# Parses exactly `true` or `false`.
# @complexity O(N)
# @throws invalidArgument for every other spelling
# @example
#   enabled := try strconv.parseBool("true")
pub parseBool(s str) !bool:
    if strings.compare(s, "true"):
        ret true
    elif strings.compare(s, "false"):
        ret false
    ..
    throw errors.invalidArgument("invalid boolean")
..

# Formats an unsigned integer as an owned decimal string.
# @complexity O(log₁₀(value)); O(1) for zero
# @ownership Release the returned string with a.
# @example
#   text := try strconv.formatUint(a, 42)
pub formatUint(a alc.Allocator, value u64) !$str:
    remaining := value
    digits u64 = 1
    tmp := value
    while tmp >= 10:
        tmp = tmp / 10
        digits = digits + 1
    ..
    result str = try strings.alloc(a, digits)
    out u8* = strings.toPtr(result)
    i := digits
    while i > 0:
        i = i - 1
        out[i] = cast.u64to8((remaining % 10) + 48)
        remaining = remaining / 10
    ..
    ret result
..
