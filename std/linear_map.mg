mod linear_map
# Compact string maps optimized for small collections and linear lookup.

use "std:allocator" alc
use "std:strings" stg
use "std:errors" err
use "std:cast" cast
use "std:memory" mem
use "std:slices" slices

# Owning insertion-ordered string map optimized for small collections. Keys are
# copied and lookup is linear; deletion may change entry order.
pub LinearMap[T](
    allocator alc.Allocator
    keys str*
    values T*
    cleanup ($T) void
    countValue u16
    capacity u16
)

release[T](cleanup ($T) void, value $T) void:
    a := ctx.procAlloc
    if cleanup == none:
        abandoned := array T[1]
        abandoned[0] = move value
        ret
    ..
    cleanup(move value)
..

# Creates an empty map and optionally installs a value cleanup callback.
# @complexity O(1), excluding allocation
# @ownership The map owns inserted values and must be freed.
# @example
#   map := try linear_map.new[Value](freeValue)
pub new[T](cleanup ($T) void) !$LinearMap[T]:
    a := ctx.procAlloc
    keys str* = try a.allocT[str](8)
    onerror a.free(keys)
    values T* = try a.allocT[T](8)
    ret LinearMap[T](
        allocator=a,
        keys=keys,
        values=values,
        cleanup=cleanup,
        countValue=0,
        capacity=8,
    )
..

# Returns the current index of key.
# @throws failure if key is absent
# @complexity O(N)
# @example
#   index := try map.indexOf("name")
LinearMap[T].indexOf(key str) !u64:
    # SAFETY: keys points to capacity slots and countValue never exceeds capacity.
    unsafe:
    bound := cast.u16to64(this.countValue)
    keys str* = this.keys
    for i u64 = 0 to bound:
        if stg.compare(key, keys[i]):
            ret i
        ..
    ..
      throw err.failure("key not found in linear map")
    ..
..

# Expands storage while preserving current entry order.
# @throws wouldOverflow after the map reaches 65,535 entries
# @complexity O(N)
# @example
#   try map.grow()
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
    onerror this.allocator.free(newKeys)
    newValues T* = try this.allocator.allocT[T](newCapacity)
    count := cast.u16to64(this.countValue)
    mem.copy(this.keys, newKeys, count * sizeof str)
    mem.copy(this.values, newValues, count * sizeof T)
    this.allocator.free(this.keys)
    this.allocator.free(this.values)
    this.keys = newKeys
    this.values = newValues
    this.capacity = cast.u64to16(newCapacity)
..

# Removes key and releases its value through the cleanup callback.
# @throws failure if key is absent
# @complexity O(N)
# @example
#   try map.delete("temporary")
LinearMap[T].delete(key str) !void:
    value := try this.take(key)
    release[T](this.cleanup, move value)
..

# Removes key and transfers its value to the caller without cleanup.
# @ownership The caller becomes responsible for the returned value.
# @complexity O(N)
# @example
#   value := try map.take("name")
LinearMap[T].take(key str) !$T:
    # SAFETY: indexOf returns an occupied slot below countValue; clearing a slot
    # before moving the last entry preserves unique ownership and occupancy.
    unsafe:
    idx := try this.indexOf(key)
    lastIdx := cast.u16to64(this.countValue) - 1
    keys str* = this.keys
    values T* = this.values
    taken $T = values[idx]
    values[idx] = mem.zeroValue[T]()
    keys[idx].free(this.allocator)
    if idx != lastIdx:
        keys[idx] = keys[lastIdx]
        values[idx] = values[lastIdx]
        keys[lastIdx] = ""
        values[lastIdx] = mem.zeroValue[T]()
    ..
    this.countValue = this.countValue - 1
      ret move taken
    ..
..

# Returns the value for key without removing it.
# @ownership The returned value is borrowed from the map.
# @complexity O(N)
# @example
#   value := try map.get("name")
LinearMap[T].get(key str) !T:
    # SAFETY: indexOf returns an initialized value slot below countValue.
    unsafe:
    idx := try this.indexOf(key)
    values T* = this.values
      ret values[idx]
    ..
..

# Returns the number of entries.
# @complexity O(1)
# @example
#   size := map.count()
LinearMap[T].count() u64:
    ret cast.u16to64(this.countValue)
..

# Returns a volatile borrowed view of keys in current storage order.
# @warning Any mutation can invalidate the view.
# @complexity O(1)
# @example
#   keys := map.keysView()
LinearMap[T].keysView() str[]:
    ret slices.fromPtr(this.keys, cast.u16to64(this.countValue))
..

# Returns a volatile borrowed view of values matching keysView() by index.
# @warning Any mutation can invalidate the view.
# @complexity O(1)
# @example
#   values := map.valuesView()
LinearMap[T].valuesView() T[]:
    ret slices.fromPtr(this.values, cast.u16to64(this.countValue))
..

# Inserts item under a copied key or replaces and cleans up the old value.
# @ownership Always consumes item, including when growth or key copying fails.
# @complexity O(N), including lookup
# @example
#   try map.set("name", value)
LinearMap[T].set(key str, item $T) !void:
    # SAFETY: existing indices are occupied; growth reserves capacity before a
    # new key/value pair is transferred into the next unoccupied slot.
    unsafe:
    onerror release[T](this.cleanup, move item)
    idx u64, e error = this.indexOf(key)
    if e.ok():
        existingValues T* = this.values
        previous $T = existingValues[idx]
        existingValues[idx] = mem.zeroValue[T]()
        release[T](this.cleanup, move previous)
        existingValues[idx] = move item
        ret
    ..
    if this.countValue == this.capacity:
        try growForInsert[T](this)
    ..
    ownedKey str = try stg.copy(key)
    insertAt := cast.u16to64(this.countValue)
    keys str* = this.keys
    values T* = this.values
    keys[insertAt] = move ownedKey
    values[insertAt] = move item
      this.countValue = this.countValue + 1
    ..
..

growForInsert[T](map LinearMap[T]*) !bool:
    try map.grow()
    ret true
..

# Frees copied keys, owned values, and all backing storage.
# @complexity O(N)
# @example
#   map.free()
destr LinearMap[T].free() void:
    # SAFETY: slots below countValue are initialized; keys and configured values
    # are consumed exactly once before both backing allocations are released.
    unsafe:
    bound := cast.u16to64(this.countValue)
    keys str* = this.keys
    values T* = this.values
    for i u64 = 0 to bound:
        keys[i].free(this.allocator)
    ..
    if this.cleanup != none:
        for i u64 = 0 to bound:
            value $T = values[i]
            values[i] = mem.zeroValue[T]()
            this.cleanup(move value)
        ..
    ..
    this.allocator.free(this.keys)
    this.allocator.free(this.values)
    this.keys = none
    this.values = none
    this.countValue = 0
      this.capacity = 0
    ..
..

# Removes all entries and returns the map to its initial capacity.
# @complexity O(N)
# @ownership Releases every stored value through cleanup.
# @example
#   try map.clear()
LinearMap[T].clear() !void:
    replacement := try new[T](this.cleanup)
    this.free()
    *this = move replacement
..
