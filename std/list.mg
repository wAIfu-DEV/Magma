mod list

use "allocator.mg" alc
use "slices.mg"    slc
use "cast.mg"      cast
use "errors.mg"    err
use "memory.mg"    mem

List(
    data ptr,
    contenance u16,
    padRight u16,
    padLeft u8,
    typeSize u8,
)

# data       8B
# contenance 2B
# padRight   2B
# padLeft    1B
# typeSize   1B
# Total      14B packed
# Total pad  16B padded

# List limitations:
# 1. Cannot host element of type > 255B (2040b)
# 2. Max element count of U16_MAX (65,535)
# 3. Pre allocated buffer at start of list < 255 elements (fine in virtually all scenarios)
# 4. Pre allocated buffer at end of list < 65,535 elements (255 would have been fine too)
# 5. Used size must be computed on demand.

# Current language Limitations
# 1. (2026-06) No templated types / functions

pub new(a alc.Allocator, typeSize u64) !$List:
    l List

    if typeSize > 255:
        # TODO: implement BigList
        throw err.errInvalidArgument("List cannot host elements with size > 255B")
    ..

    l.typeSize = cast.u64to8(typeSize)
    l.contenance = 8 # 4*2 buffers front and end
    l.padLeft = 4
    l.padRight = 4

    l.data = try a.alloc(typeSize * 8)
    ret l
..

List.count() u64:
    ret cast.u16to64(this.contenance) - cast.u8to64(this.padLeft) - cast.u16to64(this.padRight)
..

List.clearShrink(a alc.Allocator) !void:
    typeSize u64 = cast.u8to64(this.typeSize)
    this.data = try a.realloc(this.data, typeSize * 8)

    this.contenance = 8
    this.padLeft = 4
    this.padRight = 4
..

List.clearKeep(a alc.Allocator) !void:
    if this.contenance < 8:
        typeSize u64 = cast.u8to64(this.typeSize)
        this.data = try a.realloc(this.data, typeSize * 8)
        this.contenance = 8
    ..

    this.padLeft = 4
    this.padRight = this.contenance - 4
..

List.resize(a alc.Allocator, usable u16, padLeft u8, padRight u16) !void:
    newCont u64 = cast.u16to64(usable) + cast.u8to64(padLeft) + cast.u16to64(padRight)
    
    if newCont > 65535:
        throw err.errWouldOverflow("List cannot contain more than 65535 elements.")
    ..
    
    newSize u64 = newCont * cast.u8to64(this.typeSize)

    newData ptr = try a.alloc(newSize)

    reg0 u64 = cast.ptou(this.data) + (cast.u8to64(this.padLeft) * cast.u8to64(this.typeSize))
    reg1 u64 = cast.ptou(newData) + (cast.u8to64(padLeft) * cast.u8to64(this.typeSize))
    
    count u64 = this.count()
    if count > cast.u16to64(usable):
        count = cast.u16to64(usable)
    ..
    nBytes u64 = count * cast.u8to64(this.typeSize)

    mem.copy(cast.utop(reg0), cast.utop(reg1), nBytes)

    a.free(this.data)

    this.data = newData
    this.contenance = usable + cast.u8to64(cast.u64to16(padLeft)) + padRight
    this.padLeft = padLeft
    this.padRight = padRight
..

# Allows typed get/set once consumer adds type information to the slice.
List.view() slice:
    startIdx u64 = cast.u8to64(this.padLeft) * cast.u8to64(this.typeSize)
    viewPtr ptr = cast.utop(cast.ptou(this.data) + startIdx)
    ret slc.fromPtr(viewPtr, this.count())
..

List.expandRight(a alc.Allocator) !u64:
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

    this.data = try a.realloc(this.data, expanded * cast.u8to64(this.typeSize))
    this.contenance = cast.u64to16(expanded)
    this.padRight = cast.u64to16(newPad) - 1
    ret this.count() - 1
..

List.expandLeft(a alc.Allocator) !void:
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

    newData ptr = try a.alloc(expanded * cast.u8to64(this.typeSize))

    reg0 u64 = cast.ptou(this.data) # no padding since leftPad is 0
    reg1 u64 = cast.ptou(newData) + (cast.u8to64(newPad) * cast.u8to64(this.typeSize))
    
    count u64 = expanded - newPad - cast.u16to64(this.padRight)
    nBytes u64 = count * cast.u8to64(this.typeSize)

    mem.copy(cast.utop(reg0), cast.utop(reg1), nBytes)

    a.free(this.data)

    this.data = newData
    this.contenance = cast.u64to16(expanded)
    this.padLeft = cast.u64to8(newPad)
..

List.popRight(a alc.Allocator) !ptr:
    if this.count() == 0:
        throw err.errWouldOverflow("Cannot pop from an empty List.")
    ..

    if this.padRight > this.count() * 2:
        # shrink right padding
        # TODO: consider percentage of for padding
        try this.resize(a, this.count(), this.padLeft, 4)
    ..

    lastIdx u64 = cast.u16to64(this.contenance) - cast.u16to64(this.padRight) - 1
    elementPtr ptr = cast.utop(cast.ptou(this.data) + (lastIdx * cast.u8to64(this.typeSize)))

    this.padRight = this.padRight + 1
    ret elementPtr
..

List.popLeft(a alc.Allocator) !ptr:
    if this.count() == 0:
        throw err.errWouldOverflow("Cannot pop from an empty List.")
    ..

    if this.padLeft > this.count() * 2:
        # shrink left padding
        # TODO: consider percentage of for padding
        try this.resize(a, this.count(), 4, this.padRight)
    ..

    firstIdx u64 = cast.u8to64(this.padLeft)
    elementPtr ptr = cast.utop(cast.ptou(this.data) + (firstIdx * cast.u8to64(this.typeSize)))

    this.padLeft = this.padLeft + 1
    ret elementPtr
..

List.pushRight(a alc.Allocator, itemPtr ptr) !void:
    idx u64 = try this.expandRight(a)
    target u64 = cast.ptou(this.data) + (idx * cast.u8to64(this.typeSize))
    mem.copy(itemPtr, cast.utop(target), cast.u8to64(this.typeSize))
..

List.pushLeft(a alc.Allocator, itemPtr ptr) !void:
    try this.expandLeft(a)
    offset u64 = cast.u8to64(this.padLeft) * cast.u8to64(this.typeSize)
    destPtr ptr = cast.utop(cast.ptou(this.data) + offset)
    mem.copy(itemPtr, destPtr, cast.u8to64(this.typeSize))
..

List.free(a alc.Allocator) void:
    a.free(this.data)
    this.contenance = 0
    this.padLeft = 0
    this.padRight = 0
..
