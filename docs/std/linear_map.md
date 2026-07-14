# `std/linear_map`

A small allocator-backed string map implemented with parallel arrays and linear lookup. It preserves insertion order and copies keys it inserts.

## Type

`LinearMap[T](allocator alc.Allocator, keys arr.Array[str], values arr.Array[T])` owns key copies and array storage, but not resources inside values.

## API

- `pub new[T](a alc.Allocator) !$LinearMap[T]` creates an empty map.
- `get(key str) !T` returns a value or an error when absent.
- `set(key str, item T) !void` replaces an existing value or inserts it with an owned key copy.
- `delete(key str) !void` removes an entry and frees its copied key.
- `count() u64` returns the number of entries.
- `keysView() str[]` and `valuesView() T[]` return borrowed parallel slices. Mutating, clearing, or freeing the map may invalidate them.
- `clear() !void` removes all entries, frees keys, and retains reusable array capacity.
- `free() void` is the map's `destr` method and frees all copied keys and storage.
- `indexOf(key str) !u64` is the internal linear search routine.
