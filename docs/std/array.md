# `std/array`

## Example

```magma
a := heap.allocator()
values := try array.new[u64](a)
defer values.free(a, cast.utop(0))
try values.pushLeft(a, 5)
try values.pushRight(a, 10)
last := try values.popRight(a) # 10
```

A generic double-ended dynamic array whose allocator is supplied to each
allocating operation. Capacity, padding, and indexes use `u64`.

## Type

`Array[T](data T*, state State*)` points into one allocation containing private
capacity, size, and left-offset state followed by element storage. Cleanup
callbacks are supplied to operations that discard elements rather than stored
in the array.

## Functions

- `pub new[T](a alc.Allocator) !$Array[T]` creates an empty array with initial padding.
- `pub newWithSize[T](a alc.Allocator, usable u64, padLeft u64, padRight u64) !$Array[T]` creates an array with a zeroed usable region. Padding is clamped to minimums of 2 left and 6 right.

## Methods

- `count() u64` returns the logical element count.
- `clearShrink(a alc.Allocator, cleanup ($T) void) !void` empties the array and returns it to its initial allocation.
- `clearKeep(a alc.Allocator, cleanup ($T) void) !void` empties the array while retaining capacity.
- `resize(a alc.Allocator, usable u64, padLeft u64, padRight u64, cleanup ($T) void) !void` replaces the allocation, preserves elements that fit, and cleans up discarded values.
- `view() T[]` returns a borrowed slice of current elements. Push, pop, resize, clear, or free may invalidate it.
- `get(index u64) !T`, `take(index u64) !$T`, and `set(index u64, value $T, cleanup ($T) void) !void` access, remove, or replace indexed elements.
- `expandRight(a alc.Allocator) !u64` and `expandLeft(a alc.Allocator) !void` are growth primitives used by pushes.
- `popRight(a alc.Allocator) !$T` / `popLeft(a alc.Allocator) !$T` remove and transfer an end element; an empty array produces `wouldOverflow`.
- `pushRight(a alc.Allocator, item $T) !void` / `pushLeft(a alc.Allocator, item $T) !void` take and add an end element.
- `free(a alc.Allocator, cleanup ($T) void) void` is the array's `destr` method, cleans up remaining elements, and releases storage.
- `iterator() iter.Iterator[T]` returns an iterator over the current values.
