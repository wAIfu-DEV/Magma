mod sort
# In-place generic sorting and reversal operations for slices.

use "std:slices" slices

# Sorts a slice in ascending comparator order using stable insertion sort.
# @complexity O(N²) comparisons and swaps; O(N) for an already sorted slice
# @param in mutable slice to sort
# @param compare comparator returning a negative, zero, or positive value
# @example
#   sort.insertion(values, compare)
pub insertion[T](in T[], compare (T, T) i64) void:
    n := slices.count(in)
    for i u64 = 1 to n:
        j := i
        prev := j - 1
        # The outer loop establishes i < n; each iteration maintains j <= i,
        # and j > 0 makes prev = j - 1 a valid predecessor.
        bounded j < n, prev < n:
            loop j > 0 && compare(in[j], in[prev]) < 0:
                tmp := in[j]
                in[j] = in[prev]
                in[prev] = tmp
                j = j - 1
                prev = j - 1
            ..
        ..
    ..
..

# Reverses a slice in place.
# @complexity O(N)
# @param in mutable slice to reverse
# @example
#   sort.reverse(values)
pub reverse[T](in T[]) void:
    n := slices.count(in)
    for i u64 = 0 to n / 2:
        right := n - i - 1
        # i < n/2 and right = n-i-1 place both indices within the slice.
        bounded i < n, right < n:
            tmp := in[i]
            in[i] = in[right]
            in[right] = tmp
        ..
    ..
..
