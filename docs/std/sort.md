# `std/sort`

## Example

```magma
values := array u64[3]
values[0] = 3
values[1] = 1
values[2] = 2
sort.insertion[u64](values, compareU64)
sort.reverse[u64](values)
```

In-place generic slice ordering utilities.

- `pub insertion[T](in T[], compare (T, T) i64) void` performs a stable insertion sort using a negative/zero/positive comparator. Complexity is O(N²), making it most suitable for small or nearly sorted slices.
- `pub reverse[T](in T[]) void` reverses a slice in place in O(N).
