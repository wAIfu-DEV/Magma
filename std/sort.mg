mod sort

use "slices.mg" slices

pub insertion[T](in T[], compare (T, T) i64) void:
    n := slices.count(in)
    i u64 = 1
    while i < n:
        j := i
        prev := j - 1
        while j > 0 && compare(in[j], in[prev]) < 0:
            tmp := in[j]
            in[j] = in[prev]
            in[prev] = tmp
            j = j - 1
            prev = j - 1
        ..
        i = i + 1
    ..
..

pub reverse[T](in T[]) void:
    n := slices.count(in)
    i u64 = 0
    while i < n / 2:
        right := n - i - 1
        tmp := in[i]
        in[i] = in[right]
        in[right] = tmp
        i = i + 1
    ..
..
