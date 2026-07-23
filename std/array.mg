mod array
# Growable contiguous arrays with explicit allocator and ownership control.

# Array differs from List in the way it handles allocators
# Array does not keep an allocator while List does.
# Prefer using Array instead of List if you make use of composition

use "std:allocator" alc
use "std:slices"    slc
use "std:cast"      cast
use "std:errors"    err
use "std:memory"    mem
use "std:iterator"  iter
use "std:footgun"   fg
# Padding is biased for append-first workloads
const DEFAULT_PAD_LEFT u64 = 2
const DEFAULT_PAD_RIGHT u64 = 6
const DEFAULT_CAPACITY u64 = 8

State(
    capacity u64
    size u64
    leftOffset u64
)

State.rightBufferSize() u64:
    ret this.capacity - (this.size + this.leftOffset)
..

# Growable contiguous storage with optional padding at either end.
# Array does not retain its allocator; pass the same allocator to every operation.
pub Array[T](
    data T*,
    state State*,
)

byteSize[T](count u64) !u64:
    maxU64 u64 = 0 - 1
    if sizeof T != 0 && count > maxU64 / sizeof T:
        throw err.wouldOverflow("Array byte size overflow.")
    ..
    ret count * sizeof T
..

addSize(a u64, b u64) !u64:
    result u64 = a + b
    if result < a:
        throw err.wouldOverflow("Array allocation size overflow.")
    ..
    ret result
..

growCapacity(capacity u64) !u64:
    expanded u64 = try addSize(capacity, capacity / 2)
    ret try addSize(expanded, capacity / 4)
..

# Creates an empty array with default growth padding.
# @complexity O(1), excluding allocator cost
# @param a allocator used for initial storage
# @returns owned empty array
# @ownership Release with Array.free using the same allocator.
# @example
#   values := try array.new[u64](a)
#   try values.pushRight(a, 42)
pub new[T](a alc.Allocator) !$Array[T]:
    ret try newWithSize[T](a, 0, DEFAULT_PAD_LEFT, DEFAULT_PAD_RIGHT)
..

# Creates a zero-initialized array with explicit usable size and growth padding.
# @complexity O(usable + padLeft + padRight)
# @param a allocator used for storage
# @param usable number of initially accessible elements
# @param padLeft reserved elements before the accessible range
# @param padRight reserved elements after the accessible range
# @returns owned array
# @ownership Release with Array.free using the same allocator.
# @example
#   values := try array.newWithSize[u64](a, 8, 0, 8)
pub newWithSize[T](a alc.Allocator, usable u64, padLeft u64, padRight u64) !$Array[T]:
    if padLeft < DEFAULT_PAD_LEFT:
        padLeft = DEFAULT_PAD_LEFT
    ..
    if padRight < DEFAULT_PAD_RIGHT:
        padRight = DEFAULT_PAD_RIGHT
    ..
    capacity u64 = try addSize(usable, padLeft)
    capacity = try addSize(capacity, padRight)
    dataSize u64 = try byteSize[T](capacity)

    stateSize u64 = sizeof State
    allocationSize u64 = try addSize(stateSize, dataSize)
    headAndData u8* = try a.alloc(allocationSize)
    data u8* = cast.utop(cast.ptou(headAndData) + stateSize)

    # All elements exposed by usable must have a defined value. Zero the full
    # data region so padding that is later consumed by expand is defined too.
    mem.zero(data, dataSize)

    state State* = headAndData
    *state = State(
        capacity = capacity
        size = usable
        leftOffset = padLeft
    )

    ret Array[T](
        data = data
        state = state
    )
..

# Returns the number of accessible values.
# @complexity O(1)
# @example
#   length := values.count()
Array[T].count() u64:
    ret this.state.size
..

runCleanupFromIdx[T](arr Array[T]*, idx u64, cleanup ($T) void) void:
    if cleanup != none:
        items := arr.view()
        i u64 = idx

        zeroVal T = mem.zeroValue[T]()
        valSize u64 = sizeof T

        while i < arr.count():
            # HACK: The following tries to fix an issue introduced by the take()
            # function, if ownership can be transferred then items affected should
            # not have cleanup called on them (double free)
            # This assume that zero-initialized values are such transferred items.
            # This is shit beyond belief.

            item T* = addrof items[i]
            if mem.compare(item, addrof zeroVal, valSize) == false:
                cleanup(items[i])
            ..
            i = i + 1
        ..
    ..
..

runCleanup[T](arr Array[T]*, cleanup ($T) void) void:
    runCleanupFromIdx[T](arr, 0, cleanup)
..

# Removes every value and shrinks storage back to the default padding.
# @complexity O(N), plus cleanup cost
# @param a allocator originally used by the array
# @param cleanup callback for removed values, or none
# @example
#   try values.clearShrink(a, none)
Array[T].clearShrink(a alc.Allocator, cleanup ($T) void) !void:
    oldState := this.state

    tmp := try new[T](a)

    # Allocate first so failure leaves both the Array and its elements owned.
    runCleanup[T](this, cleanup)

    this.data = tmp.data
    this.state = tmp.state

    # new may fail, this will keep state viable in case of failure
    # hopefully O3 doesn't fuck with the ordering
    a.free(oldState)
    
    # HACK: tmp is considered owned, this drops the ownership
    # and removes compiler warnings
    fg.drop[Array[T]](tmp)
..

# Removes every value while retaining the current allocation for reuse.
# @complexity O(N), plus cleanup cost
# @param a allocator originally used by the array
# @param cleanup callback for removed values, or none
# @example
#   try values.clearKeep(a, none)
Array[T].clearKeep(a alc.Allocator, cleanup ($T) void) !void:
    if this.state.capacity < DEFAULT_CAPACITY:
        # This will reset to default size
        try this.clearShrink(a, cleanup)
        ret
    ..

    runCleanup[T](this, cleanup)

    # This will bias storage keeping to the end of the array and not the front,
    # this is good for append workloads but not so much prepend.
    # Since prepend is usually rarer than append, this seems like a sensible default.
    this.state.size = 0
    this.state.leftOffset = DEFAULT_PAD_LEFT
..

resizeStorage[T](array Array[T]*, a alc.Allocator, usable u64, padLeft u64, padRight u64, cleanup ($T) void) !void:
    newCont u64 = try addSize(usable, padLeft)
    newCont = try addSize(newCont, padRight)

    newSize u64 = try byteSize[T](newCont)

    stateSize u64 = sizeof State
    tSize u64 = sizeof T

    allocationSize u64 = try addSize(stateSize, newSize)
    newHeadAndData u8* = try a.alloc(allocationSize)
    newData ptr = cast.utop(cast.ptou(newHeadAndData) + stateSize)

    reg0 u64 = cast.ptou(array.data) + (array.state.leftOffset * tSize)
    reg1 u64 = cast.ptou(newData) + (padLeft * tSize)
    
    count u64 = array.count()
    if usable < count:
        count = usable
        runCleanupFromIdx[T](array, usable, cleanup)
    ..
    nBytes u64 = count * tSize

    mem.copy(cast.utop(reg0), cast.utop(reg1), nBytes)
    
    a.free(array.state)

    array.data = newData
    array.state = newHeadAndData

    *array.state = State(
        capacity = newCont,
        size = usable,
        leftOffset = padLeft
    )
..

# Resizes the accessible range and replaces both growth-padding regions.
# Removed values are passed to cleanup; new values are zero-initialized.
# @param a allocator originally used by the array
# @param usable requested accessible element count
# @param padLeft requested reserved capacity before the accessible range
# @param padRight requested reserved capacity after the accessible range
# @param cleanup callback for values removed by shrinking, or none
# @complexity O(N), where N is the number of values copied or cleaned up
Array[T].resize(a alc.Allocator, usable u64, padLeft u64, padRight u64, cleanup ($T) void) !void:
    oldCount u64 = this.count()
    try resizeStorage[T](this, a, usable, padLeft, padRight, cleanup)

    if usable > oldCount:
        items := this.view()
        firstNew ptr = cast.utop(cast.ptou(slc.toPtr(items)) + (oldCount * sizeof T))
        newCount u64 = usable - oldCount
        newBytes u64 = try byteSize[T](newCount)
        mem.zero(firstNew, newBytes)
    ..
..

# Returns a slice of the list's managed items.
# This generally leads to faster read / write operations than using get / set
# @warning Overwriting a slot from the view will lead to no destructor being called on the value.
# @warning any pop, push, expand operations will lead to the slice pointing to
# now invalid data. Always treat this slice as highly volatile, prefer calling
# .view() multiple times rather than caching its result.
# @complexity O(1)
# @example
#   items := values.view()
#   first := items[0]
Array[T].view() T[]:
    ret slc.fromPtr(cast.utop(cast.ptou(this.data) + (this.state.leftOffset * sizeof T)), this.state.size)
..

# Returns a borrowed copy of the value at index.
# @complexity O(1)
# @throws outOfBounds when index is outside the accessible range
# @example
#   value := try values.get(0)
Array[T].get(index u64) !T:
    if index >= this.state.size:
        throw err.outOfBounds("index is out of bounds")
    ..

    idx u64 = this.state.leftOffset + index
    typedPtr T* = cast.reinterpret[T](this.data)
    ret typedPtr[idx]
..

# Removes and returns ownership of the value at index without changing array length.
# The vacated slot is replaced with T's zero value.
# @throws outOfBounds when index is outside the accessible range
# @complexity O(1)
Array[T].take(index u64) !$T:
    if index >= this.state.size:
        throw err.outOfBounds("index is out of bounds")
    ..

    idx u64 = this.state.leftOffset + index
    typedPtr T* = cast.reinterpret[T](this.data)
    val T = typedPtr[idx]

    # Set the consumed slot as zero value, this will tell cleanup to not process
    # on later iterations.
    typedPtr[idx] = mem.zeroValue[T]()
    ret val
..

# Replaces the value at index, optionally cleaning up the previous value.
# @param index destination index
# @param value owned replacement value
# @param cleanup callback for the overwritten value, or none
# @throws outOfBounds when index is outside the accessible range
# @complexity O(1), plus cleanup cost
Array[T].set(index u64, value $T, cleanup ($T) void) !void:
    if index >= this.state.size:
        throw err.outOfBounds("index is out of bounds")
    ..

    idx u64 = this.state.leftOffset + index
    typedPtr T* = cast.reinterpret[T](this.data)
    if cleanup != none:
        cleanup(typedPtr[idx]) # cleanup overwritten slot
    ..

    typedPtr[idx] = value
    ret
..

expandRightStorage[T](array Array[T]*, a alc.Allocator) !u64:
    state := array.state
    if state.rightBufferSize() > 0:
        state.size = state.size + 1
        ret array.count() - 1
    ..
    # If no more space in back buffer, expand rightwards

    oldCont u64 = array.state.capacity
    expanded u64 = try growCapacity(oldCont) # 1.75 factor

    expandedSize u64 = try byteSize[T](expanded)
    stateSize u64 = sizeof State

    allocationSize u64 = try addSize(stateSize, expandedSize)
    array.state = try a.realloc(array.state, allocationSize)
    array.data = cast.utop(cast.ptou(array.state) + stateSize)
    array.state.capacity = expanded
    array.state.size = array.state.size + 1
    
    ret array.count() - 1
..

# Appends a zero-initialized slot and returns its index.
# @complexity Amortized O(1); O(N) when storage grows
Array[T].expandRight(a alc.Allocator) !u64:
    idx u64 = try expandRightStorage[T](this, a)
    items := this.view()
    mem.zero(addrof items[idx], sizeof T)
    ret idx
..

expandLeftStorage[T](array Array[T]*, a alc.Allocator) !void:
    state := array.state
    if state.leftOffset > 0:
        state.leftOffset = state.leftOffset - 1
        state.size = state.size + 1
        ret
    ..
    oldCont u64 = array.state.capacity
    expanded u64 = try growCapacity(oldCont) # 1.75 factor
    newPad u64 = expanded - oldCont

    try resizeStorage[T](array, a, array.state.size, newPad, array.state.rightBufferSize(), none)
    array.state.leftOffset = array.state.leftOffset - 1
    array.state.size = array.state.size + 1
..

# Prepends a zero-initialized slot.
# @complexity Amortized O(1); O(N) when storage grows
Array[T].expandLeft(a alc.Allocator) !void:
    try expandLeftStorage[T](this, a)
    items := this.view()
    mem.zero(addrof items[0], sizeof T)
..

# Removes and returns ownership of the last value.
# @complexity Amortized O(1); O(N) when storage shrinks
# @throws wouldOverflow when the array is empty
Array[T].popRight(a alc.Allocator) !$T:
    if this.state.size == 0:
        throw err.wouldOverflow("cannot pop from empty Array")
    ..

    rightBuffer u64 = this.state.rightBufferSize()
    if rightBuffer > this.state.size && rightBuffer - this.state.size > this.state.size:
        # shrink right padding
        rightPad u64 = this.state.size / 2
        if rightPad < DEFAULT_PAD_RIGHT:
            rightPad = DEFAULT_PAD_RIGHT
        ..
        try resizeStorage[T](this, a, this.state.size, this.state.leftOffset, rightPad, none)
    ..

    items T[] = this.view()
    item T = items[this.state.size - 1]

    this.state.size = this.state.size - 1
    ret item
..

# Removes and returns ownership of the first value.
# @complexity Amortized O(1); O(N) when storage shrinks
# @throws wouldOverflow when the array is empty
Array[T].popLeft(a alc.Allocator) !$T:
    if this.state.size == 0:
        throw err.wouldOverflow("Cannot pop from an empty Array.")
    ..

    if this.state.leftOffset > this.state.size && this.state.leftOffset - this.state.size > this.state.size:
        # shrink right padding
        leftPad u64 = this.state.size / 2
        if leftPad < DEFAULT_PAD_LEFT:
            leftPad = DEFAULT_PAD_LEFT
        ..
        try resizeStorage[T](this, a, this.state.size, leftPad, this.state.rightBufferSize(), none)
    ..

    items T[] = this.view()
    item T = items[0]

    this.state.leftOffset = this.state.leftOffset + 1
    this.state.size = this.state.size - 1
    
    ret item
..

# Appends an owned value to the array.
# @complexity Amortized O(1); O(N) when storage grows
# @example
#   try values.pushRight(a, 42)
Array[T].pushRight(a alc.Allocator, item $T) !void:
    idx u64 = try expandRightStorage[T](this, a)
    items T[] = this.view()
    items[idx] = item
..

# Prepends an owned value to the array.
# @complexity Amortized O(1); O(N) when storage grows
Array[T].pushLeft(a alc.Allocator, item $T) !void:
    try expandLeftStorage[T](this, a)
    items T[] = this.view()
    items[0] = item
..

# Cleans up all remaining values and releases the array storage.
# @param a allocator originally used by the array
# @param cleanup callback for each remaining value, or none
# @complexity O(N), plus cleanup cost
# @example
#   values.free(a, none)
destr Array[T].free(a alc.Allocator, cleanup ($T) void) void:
    if this.state == none:
        ret
    ..

    runCleanup[T](this, cleanup)

    a.free(this.state)
    this.state = none
    this.data = none
..

iterHasData[T](impl Array[T]*, index u64*) bool:
    bound := impl.count()
    ret (*index) < bound
..

iterNext[T](impl Array[T]*, index u64*) !T:
    bound := impl.count()
    idx := *index
    view := impl.view()
    item := view[idx]
    *index = idx + 1
    ret item
..

# Creates a non-owning iterator over the array's current accessible values.
# @warning Mutating or freeing the array invalidates the iterator.
# @complexity O(1) to create; O(1) per yielded value
Array[T].iterator() iter.Iterator[T]:
    ret iter.new[T](this, iterHasData[T], iterNext[T])
..
