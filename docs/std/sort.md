# `std/sort`

In-place generic slice ordering utilities.

- `pub insertion[T](in T[], compare (T, T) i64) void` performs a stable insertion sort using a negative/zero/positive comparator. Complexity is O(N²), making it most suitable for small or nearly sorted slices.
- `pub reverse[T](in T[]) void` reverses a slice in place in O(N).
