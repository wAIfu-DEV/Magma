# `std/iterator`

`std/iterator` provides a stateful generic iterator interface. It stores a borrowed implementation pointer, a current index, and callbacks that test for data and produce the next value.

## Type and API

- `Iterator[T](impl ptr, index u64, hasDataFunc (ptr, u64) bool, nextFunc (ptr, u64) !T)` holds the iterator state.
- `pub new[T](impl ptr, hasDataFunc (ptr, u64) bool, nextFunc (ptr, u64) !T) Iterator[T]` creates an iterator at index zero.
- `Iterator[T].hasData() bool` reports whether a value remains.
- `Iterator[T].next() !T` returns the next value and advances the iterator index after success.

The implementation pointer and referenced data must remain valid for the iterator's lifetime.

## Example

```magma
hasData(impl ptr, index u64) bool:
    ret index < 2
..

next(impl ptr, index u64) !u64:
    ret index + 10
..

values := iterator.new[u64](cast.utop(0), hasData, next)
first := try values.next() # 10
more := values.hasData()   # true
```
