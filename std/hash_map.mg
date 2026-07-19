mod hash_map

use "allocator.mg" alc
use "hash.mg" hash
use "strings.mg" strings
use "errors.mg" errors
use "memory.mg" memory
use "cast.mg" cast

HashMap[T](
    allocator alc.Allocator
    storage ptr
    capacity u64
    length u64
    cleanup ($T) void
)

keysPtr[T](map HashMap[T]*) str*:
    ret map.storage
..

valuesPtr[T](map HashMap[T]*) T*:
    ret cast.utop(cast.ptou(map.storage) + map.capacity * sizeof str)
..

statesPtr[T](map HashMap[T]*) u8*:
    ret cast.utop(cast.ptou(map.storage) + map.capacity * sizeof str + map.capacity * sizeof T)
..

valuesAt[T](storage ptr, capacity u64) T*:
    ret cast.utop(cast.ptou(storage) + capacity * sizeof str)
..

statesAt[T](storage ptr, capacity u64) u8*:
    ret cast.utop(cast.ptou(storage) + capacity * sizeof str + capacity * sizeof T)
..

storageSize[T](capacity u64) !u64:
    maxU64 u64 = 0 - 1
    if sizeof str != 0 && capacity > maxU64 / sizeof str:
        throw errors.wouldOverflow("hash map storage size overflow")
    ..
    keysBytes := capacity * sizeof str
    if sizeof T != 0 && capacity > (maxU64 - keysBytes) / sizeof T:
        throw errors.wouldOverflow("hash map storage size overflow")
    ..
    valuesBytes := capacity * sizeof T
    if capacity > maxU64 - keysBytes - valuesBytes:
        throw errors.wouldOverflow("hash map storage size overflow")
    ..
    ret keysBytes + valuesBytes + capacity
..

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

pub new[T](a alc.Allocator, capacity u64, cleanup ($T) void) !$HashMap[T]:
    if capacity == 0:
        throw errors.invalidArgument("hash map capacity must be positive")
    ..
    storage := try a.alloc(try storageSize[T](capacity))
    memory.zero(statesAt[T](storage, capacity), capacity)
    ret HashMap[T](allocator=a, storage=storage, capacity=capacity, length=0, cleanup=cleanup)
..

HashMap[T].indexOf(key str) !u64:
    keys str* = keysPtr[T](this)
    states u8* = statesPtr[T](this)
    start := hash.string(key) % this.capacity
    i u64 = 0
    while i < this.capacity:
        idx := (start + i) % this.capacity
        if states[idx] == 0:
            throw errors.failure("key not found in hash map")
        ..
        if states[idx] == 1 && strings.compare(keys[idx], key):
            ret idx
        ..
        i = i + 1
    ..
    throw errors.failure("key not found in hash map")
..

HashMap[T].get(key str) !T:
    idx := try this.indexOf(key)
    values T* = valuesPtr[T](this)
    ret values[idx]
..

# Rebuilds the table at a larger capacity. Owned keys and values are moved
# without copying.
HashMap[T].resize(newCapacity u64) !void:
    if newCapacity <= this.length:
        throw errors.invalidArgument("hash map capacity is too small")
    ..

    newStorage ptr = try this.allocator.alloc(try storageSize[T](newCapacity))
    keys str* = newStorage
    values T* = valuesAt[T](newStorage, newCapacity)
    states u8* = statesAt[T](newStorage, newCapacity)
    memory.zero(states, newCapacity)

    oldKeys str* = keysPtr[T](this)
    oldValues T* = valuesPtr[T](this)
    oldStates u8* = statesPtr[T](this)

    i u64 = 0
    while i < this.capacity:
        if oldStates[i] == 1:
            start := hash.string(oldKeys[i]) % newCapacity
            probe u64 = 0
            while probe < newCapacity:
                idx := (start + probe) % newCapacity
                if states[idx] == 0:
                    keys[idx] = oldKeys[i]
                    values[idx] = oldValues[i]
                    states[idx] = 1
                    break
                ..
                probe = probe + 1
            ..
        ..
        i = i + 1
    ..

    this.allocator.free(this.storage)
    this.storage = newStorage
    this.capacity = newCapacity
..

resizeForInsert[T](map HashMap[T]*, newCapacity u64) !bool:
    try map.resize(newCapacity)
    ret true
..

HashMap[T].set(key str, item $T) !void:
    # Keep the load factor below 75%. Besides maintaining probe performance,
    # rebuilding also discards tombstones left by delete().
    if (this.length + 1) * 4 >= this.capacity * 3:
        if this.capacity > 9223372036854775807:
            release[T](this.cleanup, item)
            throw errors.wouldOverflow("hash map capacity overflow")
        ..
        resized bool, resizeErr error = resizeForInsert[T](this, this.capacity * 2)
        if resizeErr.nok():
            release[T](this.cleanup, item)
            throw resizeErr
        ..
    ..

    keys str* = keysPtr[T](this)
    values T* = valuesPtr[T](this)
    states u8* = statesPtr[T](this)
    start := hash.string(key) % this.capacity
    firstDeleted := this.capacity
    i u64 = 0
    while i < this.capacity:
        idx := (start + i) % this.capacity
        if states[idx] == 1 && strings.compare(keys[idx], key):
            release[T](this.cleanup, claim[T](values[idx]))
            values[idx] = item
            ret
        elif states[idx] == 2 && firstDeleted == this.capacity:
            firstDeleted = idx
        elif states[idx] == 0:
            if firstDeleted != this.capacity:
                idx = firstDeleted
            ..
            ownedKey str, copyErr error = strings.copy(this.allocator, key)
            if copyErr.nok():
                release[T](this.cleanup, item)
                throw copyErr
            ..
            keys[idx] = ownedKey
            values[idx] = item
            states[idx] = 1
            this.length = this.length + 1
            ret
        ..
        i = i + 1
    ..
    if firstDeleted != this.capacity:
        fallbackKey str, fallbackErr error = strings.copy(this.allocator, key)
        if fallbackErr.nok():
            release[T](this.cleanup, item)
            throw fallbackErr
        ..
        keys[firstDeleted] = fallbackKey
        values[firstDeleted] = item
        states[firstDeleted] = 1
        this.length = this.length + 1
        ret
    ..
    release[T](this.cleanup, item)
    throw errors.wouldOverflow("hash map is full")
..

HashMap[T].delete(key str) !void:
    value := try this.take(key)
    release[T](this.cleanup, value)
..

HashMap[T].take(key str) !$T:
    idx := try this.indexOf(key)
    keys str* = keysPtr[T](this)
    values T* = valuesPtr[T](this)
    states u8* = statesPtr[T](this)
    taken := claim[T](values[idx])
    strings.free(this.allocator, keys[idx])
    states[idx] = 2
    this.length = this.length - 1
    ret taken
..

HashMap[T].count() u64:
    ret this.length
..

destr HashMap[T].free() void:
    keys str* = keysPtr[T](this)
    states u8* = statesPtr[T](this)
    i u64 = 0
    while i < this.capacity:
        if states[i] == 1:
            strings.free(this.allocator, keys[i])
        ..
        i = i + 1
    ..
    if this.cleanup != none:
        values T* = valuesPtr[T](this)
        i = 0
        while i < this.capacity:
            if states[i] == 1:
                this.cleanup(values[i])
            ..
            i = i + 1
        ..
    ..
    this.allocator.free(this.storage)
    this.storage = none
    this.capacity = 0
    this.length = 0
..
