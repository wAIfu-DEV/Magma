# `std/list`

## Example

```magma
values := try list.new[u64](heap.allocator(), cast.utop(0))
defer values.free()
try values.pushRight(4)
try values.pushLeft(2)
first := try values.popLeft() # 2
```

A generic double-ended dynamic list that retains its allocator. It delegates storage to `std/array` and shares its 65,535-slot limit.

## Type

`List[T](allocator alc.Allocator, array arr.Array[T])` owns its backing array.

## API

- `pub new[T](a alc.Allocator, cleanup ($T) void) !$List[T]` creates an empty list with an optional element cleanup callback.
- `pub fromArray[T](a alc.Allocator, array arr.Array[T]) $List[T]` transfers an existing array into a list; use the allocator that owns that array.
- `count() u64` returns the element count.
- `clearShrink() !void` empties and shrinks to initial capacity; `clearKeep() !void` empties while retaining capacity.
- `resize(usable u16, padLeft u16, padRight u16) !void` changes capacity/padding while preserving elements that fit.
- `view() T[]` returns a borrowed element slice. Any structural mutation may invalidate it.
- `pushRight(item $T) !void`, `pushLeft(item $T) !void`, `popRight() !$T`, and `popLeft() !$T` transfer elements at either end; popping an empty list fails.
- `expandRight() !u64` and `expandLeft() !void` are low-level growth methods.
- `free() void` is the list's `destr` method and releases storage.
- `iterator() iter.Iterator[T]` returns an iterator over the current values.
