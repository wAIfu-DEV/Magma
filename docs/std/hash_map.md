# `std/hash_map`

An allocator-backed, open-addressed map from borrowed string keys to values of `T`.

## Type

`HashMap[T]` stores its allocator, key and value arrays, slot states, capacity, and logical length. Keys are copied and owned by the map; values are not copied or recursively freed.

## API

- `pub new[T](a alc.Allocator, capacity u64) !$HashMap[T]` creates an empty table. Capacity must be nonzero.
- `HashMap[T].get(key str) !T` returns the associated value or an error when absent.
- `HashMap[T].set(key str, value T) !void` inserts or replaces an entry and grows the table as needed. New keys are copied using the map's allocator.
- `HashMap[T].delete(key str) !void` removes an entry or errors when absent.
- `HashMap[T].count() u64` returns the number of live entries.
- `HashMap[T].free() void` releases table storage and owned key strings, but not resources contained in values.

`indexOf(key str) !u64` is the internal probe routine. `resize(newCapacity u64) !void` rebuilds the table without copying keys or values.
