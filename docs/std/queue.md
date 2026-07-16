# `std/queue`

## Example

```magma
values := try queue.new[u64](heap.allocator(), cast.utop(0))
defer values.free()
try values.enqueue(3)
try values.enqueue(7)
first := try values.dequeue() # 3
```

An allocator-backed generic FIFO queue.

## Type

`Queue[T](allocator alc.Allocator, array arr.Array[T])` owns a double-ended backing array.

## API

- `pub new[T](a alc.Allocator, cleanup ($T) void) !$Queue[T]` creates an empty queue with an optional element cleanup callback.
- `enqueue(item $T) !void` takes an item and appends it to the back.
- `dequeue() !$T` removes and transfers the front item; an empty queue fails.
- `view() T[]` returns a borrowed FIFO-order slice, invalidated by structural mutation.
- `count() u64` returns the number of items.
- `clear() !void` empties the queue while retaining reusable capacity.
- `free() void` is the queue's `destr` method and releases backing storage.
