mod list

# Array differs from List in the way it handles allocators
# Array does not keep an allocator while List does,
# This means List will always have a larger memory footprint than array.
# Prefer using Array instead of List if you make use of composition

use "allocator.mg" alc
use "array.mg"     arr
use "iterator.mg"  iter

List[T](
    allocator alc.Allocator
    array     arr.Array[T]
    cleanup   ($T) void
)

pub new[T](a alc.Allocator, cleanup ($T) void) !$List[T]:
	array := try arr.new[T](a)
	ret List[T](
        allocator=a,
        array=array,
        cleanup=cleanup,
    )
..

pub fromArray[T](a alc.Allocator, array $arr.Array[T], cleanup ($T) void) $List[T]:
	ret List[T](allocator=a, array=array, cleanup=cleanup)
..

List[T].count() u64:
    ret this.array.count()
..

List[T].clearShrink() !void:
    try this.array.clearShrink(this.allocator, this.cleanup)
..

List[T].clearKeep() !void:
    try this.array.clearKeep(this.allocator, this.cleanup)
..

List[T].resize(usable u64, padLeft u64, padRight u64) !void:
    try this.array.resize(this.allocator, usable, padLeft, padRight, this.cleanup)
..

# Returns a slice of the list's managed items.
# Warning: any pop, push, expand operations will lead to the slice pointing to
# now invalid data. Always treat this slice as highly volatile, prefer calling
# .view() multiple times rather than caching its result.
List[T].view() T[]:
    ret this.array.view()
..

List[T].get(index u64) !T:
    ret try this.array.get(index)
..

List[T].take(index u64) !$T:
    ret try this.array.take(index)
..

List[T].set(index u64, value $T) !void:
    ret try this.array.set(index, value, this.cleanup)
..

List[T].expandRight() !u64:
    ret try this.array.expandRight(this.allocator)
..

List[T].expandLeft() !void:
    ret try this.array.expandLeft(this.allocator)
..

List[T].popRight() !$T:
    ret try this.array.popRight(this.allocator)
..

List[T].popLeft() !$T:
    ret try this.array.popLeft(this.allocator)
..

List[T].pushRight(item $T) !void:
    try this.array.pushRight(this.allocator, item)
..

List[T].pushLeft(item $T) !void:
    try this.array.pushLeft(this.allocator, item)
..

destr List[T].free() void:
    this.array.free(this.allocator, this.cleanup)
..

List[T].iterator() iter.Iterator[T]:
    ret this.array.iterator()
..
