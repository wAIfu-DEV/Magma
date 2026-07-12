mod hash_map

use "allocator.mg" alc
use "hash.mg" hash
use "strings.mg" strings
use "errors.mg" errors
use "memory.mg" memory

HashMap[T](
    allocator alc.Allocator
    keys ptr
    values ptr
    states ptr
    capacity u64
    length u64
)

pub new[T](a alc.Allocator, capacity u64) !$HashMap[T]:
    if capacity == 0:
        throw errors.invalidArgument("hash map capacity must be positive")
    ..
    m HashMap[T]
    m.allocator = a
    m.capacity = capacity
    m.keys = try a.alloc(capacity * sizeof str)
    m.values = try a.alloc(capacity * sizeof T)
    m.states = try a.alloc(capacity)
    memory.zero(m.states, capacity)
    ret m
..

HashMap[T].indexOf(key str) !u64:
    keys str* = this.keys
    states u8* = this.states
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
    values T* = this.values
    ret values[idx]
..

# Rebuilds the table at a larger capacity. Owned keys and values are moved
# without copying.
HashMap[T].resize(newCapacity u64) !void:
    if newCapacity <= this.length:
        throw errors.invalidArgument("hash map capacity is too small")
    ..

    newKeys u8*, keysErr error = this.allocator.alloc(newCapacity * sizeof str)
    if errors.code(keysErr) != 0:
        throw keysErr
    ..

    newValues u8*, valuesErr error = this.allocator.alloc(newCapacity * sizeof T)
    if errors.code(valuesErr) != 0:
        this.allocator.free(newKeys)
        throw valuesErr
    ..

    newStates u8*, statesErr error = this.allocator.alloc(newCapacity)
    if errors.code(statesErr) != 0:
        this.allocator.free(newKeys)
        this.allocator.free(newValues)
        throw statesErr
    ..
    memory.zero(newStates, newCapacity)

    oldKeys str* = this.keys
    oldValues T* = this.values
    oldStates u8* = this.states
    keys str* = newKeys
    values T* = newValues
    states u8* = newStates

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

    this.allocator.free(this.keys)
    this.allocator.free(this.values)
    this.allocator.free(this.states)
    this.keys = newKeys
    this.values = newValues
    this.states = newStates
    this.capacity = newCapacity
..

HashMap[T].set(key str, value T) !void:
    # Keep the load factor below 75%. Besides maintaining probe performance,
    # rebuilding also discards tombstones left by delete().
    if (this.length + 1) * 4 >= this.capacity * 3:
        if this.capacity > 9223372036854775807:
            throw errors.wouldOverflow("hash map capacity overflow")
        ..
        try this.resize(this.capacity * 2)
    ..

    keys str* = this.keys
    values T* = this.values
    states u8* = this.states
    start := hash.string(key) % this.capacity
    firstDeleted := this.capacity
    i u64 = 0
    while i < this.capacity:
        idx := (start + i) % this.capacity
        if states[idx] == 1 && strings.compare(keys[idx], key):
            values[idx] = value
            ret
        elif states[idx] == 2 && firstDeleted == this.capacity:
            firstDeleted = idx
        elif states[idx] == 0:
            if firstDeleted != this.capacity:
                idx = firstDeleted
            ..
            keys[idx] = try strings.copy(this.allocator, key)
            values[idx] = value
            states[idx] = 1
            this.length = this.length + 1
            ret
        ..
        i = i + 1
    ..
    if firstDeleted != this.capacity:
        keys[firstDeleted] = try strings.copy(this.allocator, key)
        values[firstDeleted] = value
        states[firstDeleted] = 1
        this.length = this.length + 1
        ret
    ..
    throw errors.wouldOverflow("hash map is full")
..

HashMap[T].delete(key str) !void:
    idx := try this.indexOf(key)
    keys str* = this.keys
    states u8* = this.states
    strings.free(this.allocator, keys[idx])
    states[idx] = 2
    this.length = this.length - 1
..

HashMap[T].count() u64:
    ret this.length
..

HashMap[T].free() void:
    keys str* = this.keys
    states u8* = this.states
    i u64 = 0
    while i < this.capacity:
        if states[i] == 1:
            strings.free(this.allocator, keys[i])
        ..
        i = i + 1
    ..
    this.allocator.free(this.keys)
    this.allocator.free(this.values)
    this.allocator.free(this.states)
    this.capacity = 0
    this.length = 0
..
