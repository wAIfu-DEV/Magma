mod linear_map

use "allocator.mg" alc
use "strings.mg" stg
use "errors.mg" err
use "cast.mg" cast
use "memory.mg" mem
use "slices.mg" slices

LinearMap[T](
    allocator alc.Allocator
    keys str*
    values T*
    cleanup ($T) void
    countValue u16
    capacity u16
)

release[T](cleanup ($T) void, value $T) void:
    if cleanup == none:
        abandoned T[1]
        abandoned[0] = value
        ret
    ..
    cleanup(value)
..

claim[T](claimed $T) $T:
    ret claimed
..

pub new[T](a alc.Allocator, cleanup ($T) void) !$LinearMap[T]:
    keys str* = try a.allocT[str](8)
    valuesRaw T*, valuesErr error = a.allocT[T](8)
    if valuesErr.nok():
        a.free(keys)
        throw valuesErr
    ..
    values T* = valuesRaw
    ret LinearMap[T](
        allocator=a,
        keys=keys,
        values=values,
        cleanup=cleanup,
        countValue=0,
        capacity=8,
    )
..

LinearMap[T].indexOf(key str) !u64:
    bound := cast.u16to64(this.countValue)
    keys str* = this.keys
    i u64 = 0
    while i < bound:
        if stg.compare(key, keys[i]):
            ret i
        ..
        i = i + 1
    ..
    throw err.failure("key not found in linear map")
..

LinearMap[T].grow() !void:
    oldCapacity := cast.u16to64(this.capacity)
    if oldCapacity >= 65535:
        throw err.wouldOverflow("linear map cannot contain more than 65535 entries")
    ..
    newCapacity := oldCapacity * 2
    if newCapacity > 65535:
        newCapacity = 65535
    ..
    newKeys str* = try this.allocator.allocT[str](newCapacity)
    newValuesRaw T*, valuesErr error = this.allocator.allocT[T](newCapacity)
    if valuesErr.nok():
        this.allocator.free(newKeys)
        throw valuesErr
    ..
    newValues T* = newValuesRaw
    count := cast.u16to64(this.countValue)
    mem.copy(this.keys, newKeys, count * sizeof str)
    mem.copy(this.values, newValues, count * sizeof T)
    this.allocator.free(this.keys)
    this.allocator.free(this.values)
    this.keys = newKeys
    this.values = newValues
    this.capacity = cast.u64to16(newCapacity)
..

LinearMap[T].delete(key str) !void:
    value := try this.take(key)
    release[T](this.cleanup, value)
..

LinearMap[T].take(key str) !$T:
    idx := try this.indexOf(key)
    lastIdx := cast.u16to64(this.countValue) - 1
    keys str* = this.keys
    values T* = this.values
    taken := claim[T](values[idx])
    stg.free(this.allocator, keys[idx])
    if idx != lastIdx:
        keys[idx] = keys[lastIdx]
        values[idx] = values[lastIdx]
    ..
    this.countValue = this.countValue - 1
    ret taken
..

LinearMap[T].get(key str) !T:
    idx := try this.indexOf(key)
    values T* = this.values
    ret values[idx]
..

LinearMap[T].count() u64:
    ret cast.u16to64(this.countValue)
..

LinearMap[T].keysView() str[]:
    ret slices.fromPtr(this.keys, cast.u16to64(this.countValue))
..

LinearMap[T].valuesView() T[]:
    ret slices.fromPtr(this.values, cast.u16to64(this.countValue))
..

LinearMap[T].set(key str, item $T) !void:
    idx u64, e error = this.indexOf(key)
    if e.ok():
        existingValues T* = this.values
        release[T](this.cleanup, claim[T](existingValues[idx]))
        existingValues[idx] = item
        ret
    ..
    if this.countValue == this.capacity:
        grown bool, growErr error = growForInsert[T](this)
        if growErr.nok():
            release[T](this.cleanup, item)
            throw growErr
        ..
    ..
    ownedKey str, copyErr error = stg.copy(this.allocator, key)
    if copyErr.nok():
        release[T](this.cleanup, item)
        throw copyErr
    ..
    insertAt := cast.u16to64(this.countValue)
    keys str* = this.keys
    values T* = this.values
    keys[insertAt] = ownedKey
    values[insertAt] = item
    this.countValue = this.countValue + 1
..

growForInsert[T](map LinearMap[T]*) !bool:
    try map.grow()
    ret true
..

destr LinearMap[T].free() void:
    i u64 = 0
    bound := cast.u16to64(this.countValue)
    keys str* = this.keys
    values T* = this.values
    while i < bound:
        stg.free(this.allocator, keys[i])
        i = i + 1
    ..
    if this.cleanup != none:
        i = 0
        while i < bound:
            this.cleanup(values[i])
            i = i + 1
        ..
    ..
    this.allocator.free(this.keys)
    this.allocator.free(this.values)
    this.keys = none
    this.values = none
    this.countValue = 0
    this.capacity = 0
..

LinearMap[T].clear() !void:
    replacement := try new[T](this.allocator, this.cleanup)
    this.free()
    this.keys = replacement.keys
    this.values = replacement.values
    this.countValue = replacement.countValue
    this.capacity = replacement.capacity
    abandoned LinearMap[T][1]
    abandoned[0] = replacement
..
