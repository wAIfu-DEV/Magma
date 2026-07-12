mod search

use "slices.mg" slices
use "errors.mg" errors

pub linear[T](in T[], value T, compare (T, T) i64) !u64:
    i u64 = 0
    while i < slices.count(in):
        if compare(in[i], value) == 0:
            ret i
        ..
        i = i + 1
    ..
    throw errors.failure("value not found")
..

pub binary[T](in T[], value T, compare (T, T) i64) !u64:
    low u64 = 0
    high := slices.count(in)
    while low < high:
        mid := low + (high - low) / 2
        order := compare(in[mid], value)
        if order < 0:
            low = mid + 1
        elif order > 0:
            high = mid
        else:
            ret mid
        ..
    ..
    throw errors.failure("value not found")
..
