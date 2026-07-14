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

Array[T](
    data T*,
    cleanup ($T) void,
    capacity u16,
    padRight u16,
    padLeft u16,
)

byteSize[T](count u64) !u64:
    maxU64 u64 = 0 - 1
    if sizeof T != 0 && count > maxU64 / sizeof T:
        throw err.wouldOverflow("Array byte size overflow.")
    ..
    ret count * sizeof T
..

pub new[T](a alc.Allocator, cleanup ($T) void) !$Array[T]:
    l Array[T]
    l.cleanup = cleanup

    l.capacity = 8 # 4*2 buffers front and end
    l.padLeft = 4
    l.padRight = 4

    initialSize u64 = try byteSize[T](8)
    l.data = try a.alloc(initialSize)
    ret l
..

Array[T].count() u64:
    ret cast.u16to64(this.capacity - this.padLeft - this.padRight)
..

Array[T].clearShrink(a alc.Allocator) !void:
    initialSize u64 = try byteSize[T](8)
    newData ptr = try a.alloc(initialSize)
    if cast.ptou(this.cleanup) != 0:
        items := this.view()
        i u64 = 0
        while i < this.count():
            this.cleanup(items[i])
            i = i + 1
        ..
    ..
    a.free(this.data)
    this.data = newData

    this.capacity = 8
    this.padLeft = 4
    this.padRight = 4
..

Array[T].clearKeep(a alc.Allocator) !void:
    if this.capacity < 8:
        initialSize u64 = try byteSize[T](8)
        newData ptr = try a.alloc(initialSize)
        if cast.ptou(this.cleanup) != 0:
            earlyItems := this.view()
            earlyIndex u64 = 0
            while earlyIndex < this.count():
                this.cleanup(earlyItems[earlyIndex])
                earlyIndex = earlyIndex + 1
            ..
        ..
        a.free(this.data)
        this.data = newData
        this.capacity = 8
        this.padLeft = 4
        this.padRight = 4
        ret
    ..

    if cast.ptou(this.cleanup) != 0:
        items := this.view()
        i u64 = 0
        while i < this.count():
            this.cleanup(items[i])
            i = i + 1
        ..
    ..
    this.padLeft = 4
    this.padRight = this.capacity - 4
..

resizeStorage[T](array Array[T]*, a alc.Allocator, usable u16, padLeft u16, padRight u16) !void:
    newCont u64 = cast.u16to64(usable) + cast.u16to64(padLeft) + cast.u16to64(padRight)
    
    if newCont > 65535:
        throw err.wouldOverflow("Array cannot contain more than 65535 elements.")
    ..
    
    newSize u64 = try byteSize[T](newCont)
    newData ptr = try a.alloc(newSize)

    reg0 u64 = cast.ptou(array.data) + (cast.u16to64(array.padLeft) * sizeof T)
    reg1 u64 = cast.ptou(newData) + (cast.u16to64(padLeft) * sizeof T)
    
    count u64 = array.count()
    if count > cast.u16to64(usable):
        count = cast.u16to64(usable)
    ..
    nBytes u64 = count * sizeof T

    mem.copy(cast.utop(reg0), cast.utop(reg1), nBytes)

    a.free(array.data)
    array.data = newData

    array.capacity = cast.u64to16(newCont)
    array.padLeft = padLeft
    array.padRight = padRight
..

Array[T].resize(a alc.Allocator, usable u16, padLeft u16, padRight u16) !void:
    newCont u64 = cast.u16to64(usable) + cast.u16to64(padLeft) + cast.u16to64(padRight)
    if newCont > 65535:
        throw err.wouldOverflow("Array cannot contain more than 65535 elements.")
    ..
    newSize u64 = try byteSize[T](newCont)
    newData ptr = try a.alloc(newSize)

    count := this.count()
    if count > cast.u16to64(usable):
        if cast.ptou(this.cleanup) != 0:
            oldItems := this.view()
            i u64 = cast.u16to64(usable)
            while i < count:
                this.cleanup(oldItems[i])
                i = i + 1
            ..
        ..
        count = cast.u16to64(usable)
    ..

    reg0 u64 = cast.ptou(this.data) + (cast.u16to64(this.padLeft) * sizeof T)
    reg1 u64 = cast.ptou(newData) + (cast.u16to64(padLeft) * sizeof T)
    mem.copy(cast.utop(reg0), cast.utop(reg1), count * sizeof T)

    a.free(this.data)
    this.data = newData
    this.capacity = cast.u64to16(newCont)
    this.padLeft = padLeft
    this.padRight = padRight
..

# Returns a slice of the list's managed items.
# Warning: any pop, push, expand operations will lead to the slice pointing to
# now invalid data. Always treat this slice as highly volatile, prefer calling
# .view() multiple times rather than caching its result.
Array[T].view() T[]:
    startIdx u64 = cast.u16to64(this.padLeft) * sizeof T
    viewPtr ptr = cast.utop(cast.ptou(this.data) + startIdx)
    ret slc.fromPtr(viewPtr, this.count())
..

Array[T].expandRight(a alc.Allocator) !u64:
    if this.padRight > 0:
        this.padRight = this.padRight - 1
        ret this.count() - 1
    ..

    oldCont u64 = cast.u16to64(this.capacity)
    expanded u64 = (oldCont * 7) / 4 # 1.75 factor
    newPad u64 = expanded - oldCont

    if expanded > 65535:
        throw err.wouldOverflow("Array cannot contain more than 65535 elements.")
    ..

    expandedSize u64 = try byteSize[T](expanded)
    this.data = try a.realloc(this.data, expandedSize)
    this.capacity = cast.u64to16(expanded)
    this.padRight = cast.u64to16(newPad) - 1
    ret this.count() - 1
..

Array[T].expandLeft(a alc.Allocator) !void:
    if this.padLeft > 0:
        this.padLeft = this.padLeft - 1
        ret
    ..

    oldCont u64 = cast.u16to64(this.capacity)
    expanded u64 = (oldCont * 7) / 4 # 1.75 factor
    newPad u64 = expanded - oldCont

    if newPad > 255:
        # cap left pad to 255 entries
        newPad = 255
        expanded = oldCont + 255
    ..

    if expanded > 65535:
        throw err.wouldOverflow("Array cannot contain more than 65535 elements.")
    ..

    expandedSize u64 = try byteSize[T](expanded)
    newData ptr = try a.alloc(expandedSize)

    reg0 u64 = cast.ptou(this.data) # no padding since leftPad is 0
    reg1 u64 = cast.ptou(newData) + (newPad * sizeof T)
    
    count u64 = oldCont - cast.u16to64(this.padRight)
    nBytes u64 = count * sizeof T

    mem.copy(cast.utop(reg0), cast.utop(reg1), nBytes)

    a.free(this.data)
    this.data = newData

    this.capacity = cast.u64to16(expanded)
    this.padLeft = cast.u64to16(newPad)
..

Array[T].popRight(a alc.Allocator) !$T:
    if this.count() == 0:
        throw err.wouldOverflow("Cannot pop from an empty Array.")
    ..

    if this.padRight > this.count() * 2:
        # shrink right padding
        # TODO: consider percentage of count for padding
        try resizeStorage[T](this, a, cast.u64to16(this.count()), this.padLeft, 4)
    ..

    items T[] = this.view()
    item T = items[this.count() - 1]
    this.padRight = this.padRight + 1
    ret item
..

Array[T].popLeft(a alc.Allocator) !$T:
    if this.count() == 0:
        throw err.wouldOverflow("Cannot pop from an empty Array.")
    ..

    if this.padLeft > this.count() * 2:
        # shrink left padding
        # TODO: consider percentage of for padding
        try resizeStorage[T](this, a, cast.u64to16(this.count()), 4, this.padRight)
    ..

    items T[] = this.view()
    item T = items[0]
    this.padLeft = this.padLeft + 1
    ret item
..

Array[T].pushRight(a alc.Allocator, item $T) !void:
    idx u64 = try this.expandRight(a)
    items T[] = this.view()
    items[idx] = item
..

Array[T].pushLeft(a alc.Allocator, item $T) !void:
    try this.expandLeft(a)
    items T[] = this.view()
    items[0] = item
..

destr Array[T].free(a alc.Allocator) void:
    if cast.ptou(this.cleanup) != 0:
        items := this.view()
        i u64 = 0
        while i < this.count():
            this.cleanup(items[i])
            i = i + 1
        ..
    ..
    a.free(this.data)
    this.capacity = 0
    this.padLeft = 0
    this.padRight = 0
..

iterHasData[T](impl ptr, index u64*) bool:
    arrPtr Array[T]* = impl
    bound := arrPtr.count()
    idx := *index
    ret idx < bound
..

iterNext[T](impl ptr, index u64*) !T:
    arrPtr Array[T]* = impl
    bound := arrPtr.count()
    idx := *index
    view := arrPtr.view()
    item := view[idx]
    index[0] = idx + 1
    ret item
..

Array[T].iterator() iter.Iterator[T]:
    ret iter.new[T](this, iterHasData[T], iterNext[T])
..
