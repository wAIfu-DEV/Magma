mod linear_map

use "list.mg" list
use "allocator.mg" alc
use "strings.mg" stg
use "errors.mg" err
use "memory.mg" mem

LinearMap(
    keys list.List
    values list.List
    allocator alc.Allocator
    typeSize u64
)

pub new(a alc.Allocator, valueTypeSize u64) !$LinearMap:
    lm LinearMap
    lm.allocator = a
    
    lm.keys = list.new(a, sizeof str)
    lm.values = list.new(a, valueTypeSize)
    lm.typeSize = valueTypeSize
    ret lm
..

LinearMap.indexOf(key str) !u64:
    view str[] = this.keys.view()
    bound := this.keys.count()

    i u64 = 0
    while i < bound:
        if stg.compare(key, view[i]):
            ret i
        ..
        i = i + 1
    ..
    throw err.errFailure("key not found in linear map")
..

LinearMap.delete(key str) !void:
    idx := try this.indexOf(key)
    lastIdx := this.keys.count() - 1

    # is already last idx, just pop
    if this.keys.count() == 1 || idx == lastIdx:
        try this.keys.popRight(this.allocator)
        try this.values.popRight(this.allocator)
        ret
    ..

    # swap with last entry, then pop last

    keyView u8[] = this.keys.view()
    valView u8[] = this.values.view()

    key0 ptr = &keyView[idx * sizeof str]
    key1 ptr = &keyView[lastIdx * sizeof str]
    mem.swap(key0, key1, sizeof str)

    val0 ptr = &valView[idx * this.typeSize]
    val1 ptr = &valView[lastIdx * this.typeSize]
    mem.swap(val0, val1, this.typeSize)

    try this.keys.popRight(this.allocator)
    try this.values.popRight(this.allocator)
..

LinearMap.get(key str) !ptr:
    idx := try this.indexOf(key)
    view u8[] = this.values.view()
    ret &view[idx * this.typeSize]
..

LinearMap.set(key str, itemPtr ptr) !void:
    idx u64, e error = this.indexOf(key)
    if err.code(e) != 0:
        tmp str = key
        try this.keys.pushRight(this.allocator, &tmp)
        try this.values.pushRight(this.allocator, itemPtr)
    else:
        view u8[] = this.values.view()
        idxPtr u8* = &view[idx * this.typeSize]
        mem.copy(itemPtr, idxPtr, this.typeSize)
    ..
..

LinearMap.clear() !void:
    try this.keys.clearShrink(this.allocator)
    try this.values.clearShrink(this.allocator)
..

LinearMap.free() void:
    this.keys.free(this.allocator)
    this.values.free(this.allocator)
..