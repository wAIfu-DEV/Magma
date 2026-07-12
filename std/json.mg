mod json

use "allocator.mg"  alc
use "array.mg"      arr
use "cast.mg"       cast
use "errors.mg"     errors
use "linear_map.mg" linear_map
use "slices.mg"     slices
use "strings.mg"    strings
use "writer.mg"     writer

# JSON value. Payloads are stored in raw u128 storage and reinterpreted based
# on the kind tag. This keeps Value independent of its recursive payload types.
Value(
    value u128
    kind u8
)

# JSON object backed by a linear map. values are borrowed; free releases the keys
# and map storage.
Object(
    entries linear_map.LinearMap[Value]
)

# JSON array. Values are borrowed; free only releases the array storage.
Array(
    allocator alc.Allocator
    values arr.Array[Value]
)

Value.asNull() !ptr:
    if this.kind != 0:
        throw errors.invalidType("json value is not null")
    ..
    ret cast.utop(0)
..

Value.asBool() !bool:
    if this.kind != 1:
        throw errors.invalidType("json value is not bool")
    ..
    r bool* = cast.reinterpret[bool](addrof this.value)
    ret *r
..

Value.asFloat() !f64:
    if this.kind == 6:
        ret cast.itof(this.asInt())
    ..
    if this.kind != 2:
        throw errors.invalidType("json value is not float")
    ..
    r f64* = cast.reinterpret[f64](addrof this.value)
    ret *r
..

Value.asInt() !i64:
    if this.kind == 2:
        ret cast.ftoi(this.asFloat())
    ..
    if this.kind != 6:
        throw errors.invalidType("json value is not int")
    ..
    r i64* = cast.reinterpret[i64](addrof this.value)
    ret *r
..

Value.asString() !str:
    if this.kind != 3:
        throw errors.invalidType("json value is not string")
    ..
    r str* = cast.reinterpret[str](addrof this.value)
    ret *r
..

Value.asObject() !Object*:
    if this.kind != 4:
        throw errors.invalidType("json value is not object")
    ..
    r Object** = cast.reinterpret[Object*](addrof this.value)
    ret *r
..

Value.asArray() !Array*:
    if this.kind != 5:
        throw errors.invalidType("json value is not array")
    ..
    r Array** = cast.reinterpret[Array*](addrof this.value)
    ret *r
..

pub newObject(a alc.Allocator) !$Object:
    object Object
    object.entries = try linear_map.new[Value](a)
    ret object
..

pub newArray(a alc.Allocator) !$Array:
    array Array
    array.allocator = a
    array.values = try arr.new[Value](a)
    ret array
..

pub null() Value:
    value Value
    ret value
..

pub boolean(value bool) Value:
    out Value
    out.kind = 1
    r bool* = cast.reinterpret[bool](addrof out.value)
    r[0] = value
    ret out
..

pub numberFloat(value f64) Value:
    out Value
    out.kind = 2
    r f64* = cast.reinterpret[f64](addrof out.value)
    r[0] = value
    ret out
..

pub numberInt(value i64) Value:
    out Value
    out.kind = 6
    r i64* = cast.reinterpret[i64](addrof out.value)
    r[0] = value
    ret out
..

pub string(value str) Value:
    out Value
    out.kind = 3
    r str* = cast.reinterpret[str](addrof out.value)
    r[0] = value
    ret out
..

pub object(value Object*) Value:
    out Value
    out.kind = 4
    r Object** = cast.reinterpret[Object*](addrof out.value)
    r[0] = value
    ret out
..

pub array(value Array*) Value:
    out Value
    out.kind = 5
    r Array** = cast.reinterpret[Array*](addrof out.value)
    r[0] = value
    ret out
..

Object.set(key str, value Value) !void:
    try this.entries.set(key, value)
..

Object.get(key str) !Value:
    ret try this.entries.get(key)
..

Object.delete(key str) !void:
    try this.entries.delete(key)
..

Object.count() u64:
    ret this.entries.count()
..

Object.free() void:
    this.entries.free()
..

Array.append(value Value) !void:
    try this.values.pushRight(this.allocator, value)
..

Array.count() u64:
    ret this.values.count()
..

Array.free() void:
    this.values.free(this.allocator)
..

writeEscaped(w writer.Writer, value str) !void:
    single u8[1]
    single[0] = 34
    try w.writeAll(strings.fromPtrNoCopy(slices.toPtr(single), 1))
    i u64 = 0
    bound := strings.countBytes(value)
    hex str = "0123456789abcdef"
    pair u8[2]
    pair[0] = 92
    escaped u8[6]
    escaped[0] = 92
    escaped[1] = 117
    escaped[2] = 48
    escaped[3] = 48

    while i < bound:
        byte := strings.byteAt(value, i)
        if byte == 34:
            pair[1] = 34
            try w.writeAll(strings.fromPtrNoCopy(slices.toPtr(pair), 2))
        elif byte == 92:
            pair[1] = 92
            try w.writeAll(strings.fromPtrNoCopy(slices.toPtr(pair), 2))
        elif byte == 8:
            pair[1] = 98
            try w.writeAll(strings.fromPtrNoCopy(slices.toPtr(pair), 2))
        elif byte == 9:
            pair[1] = 116
            try w.writeAll(strings.fromPtrNoCopy(slices.toPtr(pair), 2))
        elif byte == 10:
            pair[1] = 110
            try w.writeAll(strings.fromPtrNoCopy(slices.toPtr(pair), 2))
        elif byte == 12:
            pair[1] = 102
            try w.writeAll(strings.fromPtrNoCopy(slices.toPtr(pair), 2))
        elif byte == 13:
            pair[1] = 114
            try w.writeAll(strings.fromPtrNoCopy(slices.toPtr(pair), 2))
        elif byte < 32:
            escaped[4] = strings.byteAt(hex, cast.u8to64(byte >> 4))
            escaped[5] = strings.byteAt(hex, cast.u8to64(byte & 15))
            try w.writeAll(strings.fromPtrNoCopy(slices.toPtr(escaped), 6))
        else:
            one ptr = cast.utop(cast.ptou(strings.toPtr(value)) + i)
            try w.writeAll(strings.fromPtrNoCopy(one, 1))
        ..
        i = i + 1
    ..
    try w.writeAll(strings.fromPtrNoCopy(slices.toPtr(single), 1))
..

finite(value f64) bool:
    valueCopy f64 = value
    bits u64* = addrof valueCopy
    ret (bits[0] & 0x7FF0000000000000) != 0x7FF0000000000000
..

writeObject(w writer.Writer, value Object*, precision u64) !void:
    try value.write(w, precision)
..

writeArray(w writer.Writer, value Array*, precision u64) !void:
    try value.write(w, precision)
..

writeValue(w writer.Writer, value Value, precision u64) !void:
    valueCopy Value = value
    if valueCopy.kind == 0:
        try w.writeAll("null")
    elif valueCopy.kind == 1:
        boolean bool* = cast.reinterpret[bool](addrof valueCopy.value)
        try w.writeBool(*boolean)
    elif valueCopy.kind == 2:
        floatNumber f64* = cast.reinterpret[f64](addrof valueCopy.value)
        if finite(*floatNumber) == false:
            throw errors.invalidArgument("JSON number must be finite")
        ..
        try w.writeFloat64(*floatNumber, precision)
    elif valueCopy.kind == 3:
        string str* = cast.reinterpret[str](addrof valueCopy.value)
        try writeEscaped(w, *string)
    elif valueCopy.kind == 4:
        object Object** = cast.reinterpret[Object*](addrof valueCopy.value)
        if cast.ptou(*object) == 0:
            throw errors.invalidArgument("JSON object pointer is null")
        ..
        try writeObject(w, *object, precision)
    elif valueCopy.kind == 5:
        array Array** = cast.reinterpret[Array*](addrof valueCopy.value)
        if cast.ptou(*array) == 0:
            throw errors.invalidArgument("JSON array pointer is null")
        ..
        try writeArray(w, *array, precision)
    elif valueCopy.kind == 6:
        intNumber i64* = cast.reinterpret[i64](addrof valueCopy.value)
        try w.writeInt64(*intNumber)
    else:
        throw errors.invalidArgument("invalid JSON value kind")
    ..
..

Value.write(w writer.Writer, precision u64) !void:
    try writeValue(w, *this, precision)
..

Object.write(w writer.Writer, precision u64) !void:
    try w.writeAll("{")
    keys := this.entries.keysView()
    values := this.entries.valuesView()
    i u64 = 0
    while i < this.count():
        if i != 0:
            try w.writeAll(",")
        ..
        try writeEscaped(w, keys[i])
        try w.writeAll(":")
        try writeValue(w, values[i], precision)
        i = i + 1
    ..
    try w.writeAll("}")
..

Array.write(w writer.Writer, precision u64) !void:
    try w.writeAll("[")
    values := this.values.view()
    i u64 = 0
    while i < this.count():
        if i != 0:
            try w.writeAll(",")
        ..
        try writeValue(w, values[i], precision)
        i = i + 1
    ..
    try w.writeAll("]")
..
