mod search
# Generic linear and binary search over slices.

use "std:slices" slices
use "std:errors" errors

# Finds the first comparator-equal value by scanning from the beginning.
# @complexity O(N)
# @param in slice to search
# @param value value to find
# @param compare comparator returning a negative, zero, or positive value
# @returns index of the first match
# @throws failure when no value compares equal
# @example
#   index := try search.linear(values, needle, compare)
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

# Finds a comparator-equal value in an ascending sorted slice.
# @complexity O(log N)
# @param in sorted slice to search
# @param value value to find
# @param compare comparator used to order the slice
# @returns index of a matching value
# @throws failure when no value compares equal
# @warning Results are undefined when in is not sorted according to compare.
# @example
#   index := try search.binary(sortedValues, needle, compare)
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
