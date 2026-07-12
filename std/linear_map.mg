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
)

pub new[T](a alc.Allocator) !$LinearMap[T]:
    lm LinearMap[T]
    lm.allocator = a
    
    lm.keys = try arr.new[str](a)
    values arr.Array[T], valuesErr error = arr.new[T](a)
    if err.code(valuesErr) != 0:
        lm.keys.free(a)
        throw valuesErr
    ..
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
    idx := try this.indexOf(key)
    lastIdx := this.keys.count() - 1

    keyView str[] = this.keys.view()
    valView T[] = this.values.view()

    stg.free(this.allocator, keyView[lastIdx])

    if idx != lastIdx:
        keyView[idx] = keyView[lastIdx]
        valView[idx] = valView[lastIdx]
    ..
    # Remove both entries together without an allocation that could fail between
    # the two state changes.
    this.keys.padRight = this.keys.padRight + 1
    this.values.padRight = this.values.padRight + 1
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

LinearMap[T].set(key str, item T) !void:
    idx u64, e error = this.indexOf(key)
    if err.code(e) != 0:
        keyIdx u64 = try this.keys.expandRight(this.allocator)
        valueIdx u64, valueErr error = this.values.expandRight(this.allocator)
        if err.code(valueErr) != 0:
            this.keys.padRight = this.keys.padRight + 1
            throw valueErr
        ..
        keyView str[] = this.keys.view()
        valueView T[] = this.values.view()
        keyView[keyIdx] = stg.copy(this.allocator, key)
        valueView[valueIdx] = item
    else:
        view T[] = this.values.view()
        view[idx] = item
    ..
..

LinearMap[T].free() void:
    i u64 = 0
    bound u64 = this.keys.count()
    view str[] = this.keys.view()

    while i < bound:
        stg.free(this.allocator, view[i])
        i = i + 1
    ..

    this.keys.free(this.allocator)
    this.values.free(this.allocator)
..


LinearMap[T].clear() !void:
    newKeys arr.Array[str] = try arr.new[str](this.allocator)
    newValues arr.Array[T], valuesErr error = arr.new[T](this.allocator)
    if err.code(valuesErr) != 0:
        newKeys.free(this.allocator)
        throw valuesErr
    ..

    this.free()
    this.keys = newKeys
    this.values = newValues
..
