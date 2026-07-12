# `std/search`

Generic slice searches. The comparator returns a negative number when its first argument is smaller, zero when equal, and a positive number when larger.

- `pub linear[T](in T[], value T, compare (T, T) i64) !u64` returns the first matching index, or an error if absent. Complexity is O(N).
- `pub binary[T](in T[], value T, compare (T, T) i64) !u64` searches a slice already sorted under `compare`, returning a matching index or an error. Complexity is O(log N).
