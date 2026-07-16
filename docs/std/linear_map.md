# `std/linear_map`

## Example

```magma
values := try linear_map.new[u64](heap.allocator(), cast.utop(0))
defer values.free()
try values.set("answer", 42)
taken := try values.take("answer")
```

A small allocator-backed string map implemented with parallel arrays and linear lookup. It preserves insertion order and copies keys it inserts.

## Type

`LinearMap[T](allocator alc.Allocator, keys arr.Array[str], values arr.Array[T])` owns key copies and array storage, but not resources inside values.

## API

- `pub new[T](a alc.Allocator, cleanup ($T) void) !$LinearMap[T]` creates an empty map with an optional value cleanup callback.
- `get(key str) !T` returns a value or an error when absent.
- `set(key str, item $T) !void` takes a value, replacing an existing value or inserting it with an owned key copy.
- `delete(key str) !void` removes an entry and frees its copied key.
- `take(key str) !$T` removes an entry and transfers its value without invoking cleanup.
- `count() u64` returns the number of entries.
- `keysView() str[]` and `valuesView() T[]` return borrowed parallel slices. Mutating, clearing, or freeing the map may invalidate them.
- `clear() !void` removes all entries, frees keys, and retains reusable array capacity.
- `free() void` is the map's `destr` method and frees all copied keys and storage.
- `indexOf(key str) !u64` is the internal linear search routine.
