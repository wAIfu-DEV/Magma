mod array

# Array differs from List in the way it handles allocators
# Array does not keep an allocator while List does.
# Prefer using Array instead of List if you make use of composition

use "allocator.mg" alc
use "slices.mg"    slc
use "cast.mg"      cast
use "errors.mg"    err
use "memory.mg"    mem
use "iterator.mg"  iter
use "footgun.mg"   fg

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

Array[T](
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

pub new[T](a alc.Allocator) !$Array[T]:
    ret try newWithSize[T](a, 0, DEFAULT_PAD_LEFT, DEFAULT_PAD_RIGHT)
..

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
# Warning 1: Overwriting a slot from the view will lead to no destructor being called on the value.
# Warning 2: any pop, push, expand operations will lead to the slice pointing to
# now invalid data. Always treat this slice as highly volatile, prefer calling
# .view() multiple times rather than caching its result.
Array[T].view() T[]:
    ret slc.fromPtr(cast.utop(cast.ptou(this.data) + (this.state.leftOffset * sizeof T)), this.state.size)
..

Array[T].get(index u64) !T:
    if index >= this.state.size:
        throw err.outOfBounds("index is out of bounds")
    ..

    idx u64 = this.state.leftOffset + index
    typedPtr T* = cast.reinterpret[T](this.data)
    ret typedPtr[idx]
..

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

Array[T].expandLeft(a alc.Allocator) !void:
    try expandLeftStorage[T](this, a)
    items := this.view()
    mem.zero(addrof items[0], sizeof T)
..

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

Array[T].pushRight(a alc.Allocator, item $T) !void:
    idx u64 = try expandRightStorage[T](this, a)
    items T[] = this.view()
    items[idx] = item
..

Array[T].pushLeft(a alc.Allocator, item $T) !void:
    try expandLeftStorage[T](this, a)
    items T[] = this.view()
    items[0] = item
..

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

# TODO: make iterator destructible in order to retain data between calls
Array[T].iterator() iter.Iterator[T]:
    ret iter.new[T](this, iterHasData[T], iterNext[T])
..
