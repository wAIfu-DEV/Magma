# `std/array`

A generic double-ended dynamic array whose allocator is supplied to each allocating operation. Capacity and padding are `u16`, so an array cannot hold more than 65,535 slots.

## Type

`Array[T](data T*, capacity u16, padRight u16, padLeft u16)` stores the allocation and unused slots on either side. Its logical count is `capacity - padLeft - padRight`.

## Functions

- `pub new[T](a alc.Allocator) !$Array[T]` creates an empty array with initial padding. The caller owns it and must call `free(a)`.
- `byteSize[T](count u64) !u64` is an internal checked `count * sizeof T` calculation.

## Methods

- `count() u64` returns the logical element count.
- `clearShrink(a alc.Allocator) !void` empties the array and returns it to its initial eight-slot allocation.
- `clearKeep(a alc.Allocator) !void` empties the array while retaining existing capacity (and ensures at least eight slots).
- `resize(a alc.Allocator, usable u16, padLeft u16, padRight u16) !void` replaces the allocation, preserves as many elements as fit, and applies the requested padding.
- `view() T[]` returns a borrowed slice of current elements. Push, pop, resize, clear, or free may invalidate it.
- `expandRight(a alc.Allocator) !u64` and `expandLeft(a alc.Allocator) !void` are growth primitives used by pushes.
- `popRight(a alc.Allocator) !T` / `popLeft(a alc.Allocator) !T` remove and return an end element; an empty array produces `wouldOverflow`.
- `pushRight(a alc.Allocator, item T) !void` / `pushLeft(a alc.Allocator, item T) !void` add an end element.
- `free(a alc.Allocator) void` releases backing storage. The allocator must match the one used for allocation.
