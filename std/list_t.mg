mod wip_list_t

use "allocator.mg" alc
use "slices.mg"    slc
use "cast.mg"      cast
use "errors.mg"    err
use "memory.mg"    mem

List[T](
    data T*,
    contenance u16,
    padRight u16,
    padLeft u16,
)

pub new[T](a alc.Allocator) !$List[T]:
    l List[T]

    l.contenance = 8 # 4*2 buffers front and end
    l.padLeft = 4
    l.padRight = 4

    l.data = try a.alloc(8 * sizeof T)
    ret l
..

List[T].count() u64:
    ret cast.u16to64(this.contenance - this.padLeft - this.padRight)
..

List[T].clearShrink(a alc.Allocator) !void:
    this.data = try a.realloc(this.data, 8 * sizeof T)

    this.contenance = 8
    this.padLeft = 4
    this.padRight = 4
..

List[T].clearKeep(a alc.Allocator) !void:
    if this.contenance < 8:
        this.data = try a.realloc(this.data, 8 * sizeof T)
        this.contenance = 8
    ..
    this.padLeft = 4
    this.padRight = this.contenance - 4
..

List[T].resize(a alc.Allocator, usable u16, padLeft u16, padRight u16) !void:
    newCont u64 = cast.u16to64(usable + padLeft + padRight)
    
    if newCont > 65535:
        throw err.errWouldOverflow("List cannot contain more than 65535 elements.")
    ..
    
    newSize u64 = newCont * sizeof T
    newData ptr = try a.alloc(newSize)

    reg0 u64 = cast.ptou(this.data) + (cast.u8to64(this.padLeft) * sizeof T)
    reg1 u64 = cast.ptou(newData) + (cast.u8to64(padLeft) * sizeof T)
    
    count u64 = this.count()
    if count > cast.u16to64(usable):
        count = cast.u16to64(usable)
    ..
    nBytes u64 = count * sizeof T

    mem.copy(cast.utop(reg0), cast.utop(reg1), nBytes)

    a.free(this.data)
    this.data = newData

    this.contenance = usable + padLeft + padRight
    this.padLeft = padLeft
    this.padRight = padRight
..

# Allows typed get/set once consumer adds type information to the slice.
List[T].view() T[]:
    startIdx u64 = cast.u16to64(this.padLeft) * sizeof T
    viewPtr ptr = cast.utop(cast.ptou(this.data) + startIdx)
    ret slc.fromPtr(viewPtr, this.count())
..

List[T].expandRight(a alc.Allocator) !u64:
    if this.padRight > 0:
        this.padRight = this.padRight - 1
        ret this.count() - 1
    ..

    oldCont u64 = cast.u16to64(this.contenance)
    expanded u64 = (oldCont * 7) / 4 # 1.75 factor
    newPad u64 = expanded - oldCont

    if expanded > 65535:
        throw err.errWouldOverflow("List cannot contain more than 65535 elements.")
    ..

    this.data = try a.realloc(this.data, expanded * sizeof T)
    this.contenance = cast.u64to16(expanded)
    this.padRight = cast.u64to16(newPad) - 1
    ret this.count() - 1
..

List[T].expandLeft(a alc.Allocator) !void:
    if this.padLeft > 0:
        this.padLeft = this.padLeft - 1
        ret
    ..

    oldCont u64 = cast.u16to64(this.contenance)
    expanded u64 = (oldCont * 7) / 4 # 1.75 factor
    newPad u64 = expanded - oldCont

    if newPad > 255:
        # cap left pad to 255 entries
        newPad = 255
        expanded = oldCont + 255
    ..

    if expanded > 65535:
        throw err.errWouldOverflow("List cannot contain more than 65535 elements.")
    ..

    newData ptr = try a.alloc(expanded * sizeof T)

    reg0 u64 = cast.ptou(this.data) # no padding since leftPad is 0
    reg1 u64 = cast.ptou(newData) + (newPad * sizeof T)
    
    count u64 = oldCont - cast.u16to64(this.padRight)
    nBytes u64 = count * sizeof T

    mem.copy(cast.utop(reg0), cast.utop(reg1), nBytes)

    a.free(this.data)
    this.data = newData

    this.contenance = cast.u64to16(expanded)
    this.padLeft = cast.u64to16(newPad)
..

List[T].popRight(a alc.Allocator) !T:
    if this.count() == 0:
        throw err.errWouldOverflow("Cannot pop from an empty List.")
    ..

    if this.padRight > this.count() * 2:
        # shrink right padding
        # TODO: consider percentage of count for padding
        try this.resize(a, this.count(), this.padLeft, 4)
    ..

    lastIdx u64 = cast.u16to64(this.contenance - this.padRight) - 1
    this.padRight = this.padRight + 1
    ret this.data[lastIdx]
..

List[T].popLeft(a alc.Allocator) !T:
    if this.count() == 0:
        throw err.errWouldOverflow("Cannot pop from an empty List.")
    ..

    if this.padLeft > this.count() * 2:
        # shrink left padding
        # TODO: consider percentage of for padding
        try this.resize(a, this.count(), 4, this.padRight)
    ..

    firstIdx u64 = cast.u16to64(this.padLeft)
    this.padLeft = this.padLeft + 1
    ret this.data[firstIdx]
..

List[T].pushRight(a alc.Allocator, item T) !void:
    idx u64 = try this.expandRight(a) # idx returned doesn't account for left padding
    this.data[cast.u16to64(this.padLeft) + idx] = item
..

List[T].pushLeft(a alc.Allocator, item T) !void:
    try this.expandLeft(a)
    this.data[cast.u16to64(this.padLeft)] = item
..

List[T].free(a alc.Allocator) void:
    a.free(this.data)
    this.contenance = 0
    this.padLeft = 0
    this.padRight = 0
..
