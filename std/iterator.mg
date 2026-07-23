mod iterator
# Type-erased pull iterators over generic values.

# Pull iterator backed by caller-provided state and callbacks.
# The iterator does not own impl or any storage referenced by it.
pub Iterator[T](
    impl ptr
    index u64

    fn_hasData (ptr, u64*) bool
    fn_next (ptr, u64*) !T
)

# Creates an iterator at index zero over caller-owned state.
# @complexity O(1)
# @param impl opaque state passed to both callbacks
# @param hasDataFunc reports whether another value is available
# @param nextFunc returns the next value and may update the index
# @ownership impl must remain valid for the iterator's entire lifetime.
# @example
#   it := iterator.new[u64](state, hasData, nextValue)
pub new[T](impl ptr, hasDataFunc (ptr, u64*) bool, nextFunc (ptr, u64*) !T) Iterator[T]:
    ret Iterator[T](impl=impl, index=0, fn_hasData=hasDataFunc, fn_next=nextFunc)
..

# Reports whether the iterator can produce another value.
# @complexity Determined by hasDataFunc
# @example
#   while it.hasData():
#       value := try it.next()
#   ..
Iterator[T].hasData() bool:
    ret this.fn_hasData(this.impl, addrof this.index)
..

# Produces the next value through the iterator callback.
# @complexity Determined by nextFunc
# @throws any error reported by nextFunc
# @warning Call hasData before next unless the callback defines other behavior.
Iterator[T].next() !T:
    ret try this.fn_next(this.impl, addrof this.index)
..
