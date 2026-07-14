# `std/queue`

An allocator-backed generic FIFO queue.

## Type

`Queue[T](allocator alc.Allocator, array arr.Array[T])` owns a double-ended backing array.

## API

- `pub new[T](a alc.Allocator) !$Queue[T]` creates an empty queue.
- `enqueue(item T) !void` appends to the back.
- `dequeue() !T` removes the front item; an empty queue fails.
- `view() T[]` returns a borrowed FIFO-order slice, invalidated by structural mutation.
- `count() u64` returns the number of items.
- `clear() !void` empties the queue while retaining reusable capacity.
- `free() void` is the queue's `destr` method and releases backing storage.
