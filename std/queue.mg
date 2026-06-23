mod queue

use "list.mg"      list
use "allocator.mg" alc

Queue(
    allocator alc.Allocator
    data list.List
)

pub new(a alc.Allocator, typeSize u64) !$Queue:
    q Queue
    q.data = try list.new(a, typeSize)
    q.allocator = a
    ret q
..

Queue.enqueue(itemPtr ptr) !void:
    try this.data.pushRight(this.allocator, itemPtr)
..

Queue.dequeue() !ptr:
    ret try this.data.popLeft(this.allocator)
..

Queue.view() slice:
    ret this.data.view()
..

Queue.count() u64:
    ret this.data.count()
..

Queue.clear() !void:
    try this.data.clearShrink(this.allocator)
..

Queue.free() void:
    this.data.free(this.allocator)
..
