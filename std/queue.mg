mod queue

use "array.mg"     arr
use "allocator.mg" alc

Queue[T](
    allocator alc.Allocator
    array     arr.Array[T]
    cleanup   ($T) void
)

pub new[T](a alc.Allocator, cleanup ($T) void) !$Queue[T]:
    array := try arr.new[T](a)
    q Queue[T]
    q.allocator = a
    q.array = array
    q.cleanup = cleanup
    ret q
..

Queue[T].enqueue(item $T) !void:
    try this.array.pushRight(this.allocator, item)
..

Queue[T].dequeue() !$T:
    ret try this.array.popLeft(this.allocator)
..

Queue[T].view() T[]:
    ret this.array.view()
..

Queue[T].count() u64:
    ret this.array.count()
..

Queue[T].clear() !void:
    try this.array.clearShrink(this.allocator, this.cleanup)
..

destr Queue[T].free() void:
    this.array.free(this.allocator, this.cleanup)
..
