# `std/hash_map`

## Example

```magma
values := try hash_map.new[u64](heap.allocator(), 8, cast.utop(0))
defer values.free()
try values.set("answer", 42)
answer := try values.get("answer")
taken := try values.take("answer")
```

An allocator-backed, open-addressed map from borrowed string keys to values of `T`.

## Type

`HashMap[T]` stores its allocator, key and value arrays, slot states, capacity, and logical length. Keys are copied and owned by the map; values are not copied or recursively freed.

## API

- `pub new[T](a alc.Allocator, capacity u64, cleanup ($T) void) !$HashMap[T]` creates an empty table. Capacity must be nonzero. The optional callback cleans up values replaced, deleted, or left at `free()`.
- `HashMap[T].get(key str) !T` returns the associated value or an error when absent.
- `HashMap[T].set(key str, value $T) !void` takes and inserts or replaces an entry and grows the table as needed. New keys are copied using the map's allocator.
- `HashMap[T].delete(key str) !void` removes an entry or errors when absent.
- `HashMap[T].take(key str) !$T` removes an entry and transfers its value to the caller without invoking cleanup.
- `HashMap[T].count() u64` returns the number of live entries.
- `HashMap[T].free() void` releases table storage and keys and invokes the configured cleanup for remaining values.

`indexOf(key str) !u64` is the internal probe routine. `resize(newCapacity u64) !void` rebuilds the table without copying keys or values.
