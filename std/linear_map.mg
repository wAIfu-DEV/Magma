mod linear_map

use "array.mg" arr
use "allocator.mg" alc
use "strings.mg" stg
use "errors.mg" err
use "cast.mg" cast

LinearMap[T](
    allocator alc.Allocator
    keys arr.Array[str]
    values arr.Array[T]
    cleanup ($T) void
)

release[T](cleanup ($T) void, value $T) void:
    if cast.ptou(cleanup) == 0:
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
    keys := try arr.new[str](a, cast.utop(0))
    values arr.Array[T], valuesErr error = arr.new[T](a, cleanup)
    if err.code(valuesErr) != 0:
        keys.free(a)
        throw valuesErr
    ..

    lm LinearMap[T]
    lm.allocator = a
    lm.cleanup = cleanup
    lm.keys = keys
    lm.values = values
    ret lm
..

LinearMap[T].indexOf(key str) !u64:
    view str[] = this.keys.view()
    bound := this.keys.count()

    i u64 = 0
    while i < bound:
        if stg.compare(key, view[i]):
            ret i
        ..
        i = i + 1
    ..
    throw err.failure("key not found in linear map")
..

LinearMap[T].delete(key str) !void:
    value := try this.take(key)
    release[T](this.cleanup, value)
..

LinearMap[T].take(key str) !$T:
    idx := try this.indexOf(key)
    lastIdx := this.keys.count() - 1

    keyView str[] = this.keys.view()
    valView T[] = this.values.view()
    taken := claim[T](valView[idx])

    stg.free(this.allocator, keyView[lastIdx])

    if idx != lastIdx:
        keyView[idx] = keyView[lastIdx]
        valView[idx] = valView[lastIdx]
    ..
    # Remove both entries together without an allocation that could fail between
    # the two state changes.
    this.keys.padRight = this.keys.padRight + 1
    this.values.padRight = this.values.padRight + 1
    ret taken
..

LinearMap[T].get(key str) !T:
    idx := try this.indexOf(key)
    view T[] = this.values.view()
    ret view[idx]
..

LinearMap[T].count() u64:
    ret this.keys.count()
..

LinearMap[T].keysView() str[]:
    ret this.keys.view()
..

LinearMap[T].valuesView() T[]:
    ret this.values.view()
..

LinearMap[T].set(key str, item $T) !void:
    idx u64, e error = this.indexOf(key)
    if err.code(e) != 0:
        keyIdx u64, keyErr error = this.keys.expandRight(this.allocator)
        if err.code(keyErr) != 0:
            release[T](this.cleanup, item)
            throw keyErr
        ..
        valueIdx u64, valueErr error = this.values.expandRight(this.allocator)
        if err.code(valueErr) != 0:
            this.keys.padRight = this.keys.padRight + 1
            release[T](this.cleanup, item)
            throw valueErr
        ..
        keyView str[] = this.keys.view()
        valueView T[] = this.values.view()
        ownedKey str, copyErr error = stg.copy(this.allocator, key)
        if err.code(copyErr) != 0:
            this.keys.padRight = this.keys.padRight + 1
            this.values.padRight = this.values.padRight + 1
            release[T](this.cleanup, item)
            throw copyErr
        ..
        keyView[keyIdx] = ownedKey
        valueView[valueIdx] = item
    else:
        view T[] = this.values.view()
        release[T](this.cleanup, claim[T](view[idx]))
        view[idx] = item
    ..
..

destr LinearMap[T].free() void:
    i u64 = 0
    bound u64 = this.keys.count()
    view str[] = this.keys.view()

    while i < bound:
        stg.free(this.allocator, view[i])
        i = i + 1
    ..

    if cast.ptou(this.cleanup) != 0:
        values := this.values.view()
        i = 0
        while i < bound:
            this.cleanup(values[i])
            i = i + 1
        ..
    ..

    this.allocator.free(this.keys.data)
    this.allocator.free(this.values.data)
..


LinearMap[T].clear() !void:
    newKeys arr.Array[str] = try arr.new[str](this.allocator, cast.utop(0))
    newValues arr.Array[T], valuesErr error = arr.new[T](this.allocator, this.cleanup)
    if err.code(valuesErr) != 0:
        newKeys.free(this.allocator)
        newValues.free(this.allocator)
        throw valuesErr
    ..

    this.free()
    this.keys = newKeys
    this.values = newValues
..
