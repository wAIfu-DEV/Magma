# `std/list`

## Example

```magma
values := try list.new[u64](heap.allocator(), cast.utop(0))
defer values.free()
try values.pushRight(4)
try values.pushLeft(2)
first := try values.popLeft() # 2
```

A generic double-ended dynamic list that retains its allocator and element
cleanup callback. It delegates storage to `std/array`.

## Type

`List[T](allocator alc.Allocator, array arr.Array[T], cleanup ($T) void)` owns
its backing array and uses `cleanup` for removed or remaining elements.

## API

- `pub new[T](a alc.Allocator, cleanup ($T) void) !$List[T]` creates an empty list with an optional element cleanup callback.
- `pub fromArray[T](a alc.Allocator, array $arr.Array[T], cleanup ($T) void) $List[T]` transfers an existing array into a list; use the allocator that owns that array.
- `count() u64` returns the element count.
- `clearShrink() !void` empties and shrinks to initial capacity; `clearKeep() !void` empties while retaining capacity.
- `resize(usable u64, padLeft u64, padRight u64) !void` changes capacity/padding while preserving elements that fit and cleans up discarded elements.
- `view() T[]` returns a borrowed element slice. Any structural mutation may invalidate it.
- `get(index u64) !T` returns an element without removing it.
- `take(index u64) !$T` removes and transfers an element without cleanup.
- `set(index u64, value $T) !void` replaces an element and cleans up the old value.
- `pushRight(item $T) !void`, `pushLeft(item $T) !void`, `popRight() !$T`, and `popLeft() !$T` transfer elements at either end; popping an empty list fails.
- `expandRight() !u64` and `expandLeft() !void` are low-level growth methods.
- `free() void` is the list's `destr` method and releases storage.
- `iterator() iter.Iterator[T]` returns an iterator over the current values.
