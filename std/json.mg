mod json
# Construction, lookup, ownership, and serialization of JSON values.

use "std:allocator"  alc
use "std:array"      arr
use "std:cast"       cast
use "std:errors"     errors
use "std:linear_map" linear_map
use "std:slices"     slices
use "std:strings"    strings
use "std:writer"     writer
use "std:memory"     memory

# JSON value. Payloads are stored in raw u128 storage and reinterpreted based
# on the kind tag. This keeps Value independent of its recursive payload types.
pub Value(
    value u128
    kind u8
    owned bool
    allocator alc.Allocator
)

# JSON object backed by a linear map. Cleanup policy for stored values is
# supplied by the caller when the object is constructed.
pub Object(
    entries linear_map.LinearMap[Value]
)

# JSON array. Cleanup policy for stored values is supplied at construction.
pub Array(
    allocator alc.Allocator
    values arr.Array[Value]
)

# Validates that this value is JSON null and returns none.
# @throws invalidType if this value has another kind
# @complexity O(1)
# @example
#   try value.asNull()
Value.asNull() !ptr:
    if this.kind != 0:
        throw errors.invalidType("json value is not null")
    ..
    ret none
..

# Returns the stored boolean.
# @throws invalidType if this value is not a boolean
# @complexity O(1)
# @example
#   enabled := try value.asBool()
Value.asBool() !bool:
    if this.kind != 1:
        throw errors.invalidType("json value is not bool")
    ..
    r bool* = cast.reinterpret[bool](addrof this.value)
    ret *r
..

# Returns a numeric value as f64, converting an integer when necessary.
# @warning Large integers can lose precision during conversion.
# @throws invalidType if this value is not numeric
# @complexity O(1)
# @example
#   ratio := try value.asFloat()
Value.asFloat() !f64:
    if this.kind == 6:
        ret cast.itof(try this.asInt())
    ..
    if this.kind != 2:
        throw errors.invalidType("json value is not float")
    ..
    r f64* = cast.reinterpret[f64](addrof this.value)
    ret *r
..

# Returns a numeric value as i64, truncating a floating-point value if necessary.
# @warning Fractional values are truncated; out-of-range conversion is target-dependent.
# @throws invalidType if this value is not numeric
# @complexity O(1)
# @example
#   count := try value.asInt()
Value.asInt() !i64:
    if this.kind == 2:
        ret cast.ftoi(try this.asFloat())
    ..
    if this.kind != 6:
        throw errors.invalidType("json value is not int")
    ..
    r i64* = cast.reinterpret[i64](addrof this.value)
    ret *r
..

# Returns the stored string as a borrowed view.
# @throws invalidType if this value is not a string
# @ownership The returned string must not outlive this value or its owning container.
# @complexity O(1)
# @example
#   name := try value.asString()
Value.asString() !str:
    if this.kind != 3:
        throw errors.invalidType("json value is not string")
    ..
    r str* = cast.reinterpret[str](addrof this.value)
    ret *r
..

# Returns the stored object pointer.
# @throws invalidType if this value is not an object
# @ownership The returned pointer is borrowed from this value.
# @complexity O(1)
# @example
#   object := try value.asObject()
Value.asObject() !Object*:
    if this.kind != 4:
        throw errors.invalidType("json value is not object")
    ..
    r Object** = cast.reinterpret[Object*](addrof this.value)
    ret *r
..

# Returns the stored array pointer.
# @throws invalidType if this value is not an array
# @ownership The returned pointer is borrowed from this value.
# @complexity O(1)
# @example
#   items := try value.asArray()
Value.asArray() !Array*:
    if this.kind != 5:
        throw errors.invalidType("json value is not array")
    ..
    r Array** = cast.reinterpret[Array*](addrof this.value)
    ret *r
..

# Returns a non-owning view of a Value. This is used by lookup operations;
# ownership is transferred out of a container only by take().
# @complexity O(1)
# @example
#   view := value.borrowed()
Value.borrowed() Value:
    out Value = *this
    out.owned = false
    ret out
..

valueCleanup(val $Value) void:
    if val.owned == false:
        ret
    ..
    if val.kind == 3:
        value str* = cast.reinterpret[str](addrof val.value)
        strings.free(val.allocator, *value)
    elif val.kind == 4:
        value Object** = cast.reinterpret[Object*](addrof val.value)
        if *value != none:
            object Object* = *value
            object.free()
        ..
    elif val.kind == 5:
        value Array** = cast.reinterpret[Array*](addrof val.value)
        if *value != none:
            array Array* = *value
            array.free()
        ..
    ..
..

# Creates an empty JSON object whose keys and owned values use allocator a.
# @complexity O(1), excluding allocation
# @ownership The returned object must be freed.
# @example
#   object := try json.newObject(a)
pub newObject(a alc.Allocator) !$Object:
    entries := try linear_map.new[Value](a, valueCleanup)
    object Object
    object.entries = entries
    ret object
..

# Creates an empty JSON array whose owned values use allocator a.
# @complexity O(1), excluding allocation
# @ownership The returned array must be freed.
# @example
#   items := try json.newArray(a)
pub newArray(a alc.Allocator) !$Array:
    values := try arr.new[Value](a)
    array Array
    array.allocator = a
    array.values = values
    ret array
..

# Creates a JSON null value.
# @complexity O(1)
# @example
#   value := json.null()
pub null() Value:
    ret memory.zeroValue[Value]()
..

# Creates a JSON boolean value.
# @complexity O(1)
# @example
#   value := json.bool(true)
pub bool(value bool) Value:
    out Value = memory.zeroValue[Value]()
    out.kind = 1
    r bool* = cast.reinterpret[bool](addrof out.value)
    *r = value
    ret out
..

# Creates a floating-point JSON number.
# @warning Serialization rejects NaN and infinities because JSON requires finite numbers.
# @complexity O(1)
# @example
#   value := json.numberFloat(3.5)
pub numberFloat(value f64) Value:
    out Value = memory.zeroValue[Value]()
    out.kind = 2
    r f64* = cast.reinterpret[f64](addrof out.value)
    *r = value
    ret out
..

# Creates an exact signed-integer JSON number.
# @complexity O(1)
# @example
#   value := json.numberInt(42)
pub numberInt(value i64) Value:
    out Value = memory.zeroValue[Value]()
    out.kind = 6
    r i64* = cast.reinterpret[i64](addrof out.value)
    *r = value
    ret out
..

# Wraps a borrowed string. The caller must keep it alive while the Value is in use.
# @complexity O(1)
# @example
#   value := json.stringBorrowed("ready")
pub stringBorrowed(value str) Value:
    out Value = memory.zeroValue[Value]()
    out.kind = 3
    r str* = cast.reinterpret[str](addrof out.value)
    *r = value
    ret out
..

# Transfers ownership of an allocated string to the returned Value.
# @ownership Consumes value; freeing the resulting owner releases it with a.
# @complexity O(1)
# @example
#   value := json.stringOwned(a, ownedText)
pub stringOwned(a alc.Allocator, value $str) Value:
    out Value = memory.zeroValue[Value]()
    out.kind = 3
    out.owned = true
    out.allocator = a
    r str* = cast.reinterpret[str](addrof out.value)
    *r = value
    ret out
..

# Copies a borrowed string and returns a Value owning the copy.
# @complexity O(N) for the string byte length
# @ownership The returned value owns its copy.
# @example
#   value := try json.stringCopy(a, input)
pub stringCopy(a alc.Allocator, value str) !$Value:
    owned str = try strings.copy(a, value)
    ret stringOwned(a, owned)
..

# Wraps a borrowed object. The caller remains responsible for freeing it.
# @complexity O(1)
# @example
#   value := json.objectBorrowed(addrof object)
pub objectBorrowed(value Object*) Value:
    out Value = memory.zeroValue[Value]()
    out.kind = 4
    r Object** = cast.reinterpret[Object*](addrof out.value)
    *r = value
    ret out
..

# Transfers responsibility for freeing the object's contents to the Value.
# The pointer storage itself remains borrowed and must outlive the Value.
# @complexity O(1)
# @example
#   value := json.objectOwned(addrof object)
pub objectOwned(value Object*) Value:
    out Value = memory.zeroValue[Value]()
    out.kind = 4
    out.owned = true
    r Object** = cast.reinterpret[Object*](addrof out.value)
    *r = value
    ret out
..

# Wraps a borrowed array. The caller remains responsible for freeing it.
# @complexity O(1)
# @example
#   value := json.arrayBorrowed(addrof items)
pub arrayBorrowed(value Array*) Value:
    out Value = memory.zeroValue[Value]()
    out.kind = 5
    r Array** = cast.reinterpret[Array*](addrof out.value)
    *r = value
    ret out
..

# Transfers responsibility for freeing the array's contents to the Value.
# The pointer storage itself remains borrowed and must outlive the Value.
# @complexity O(1)
# @example
#   value := json.arrayOwned(addrof items)
pub arrayOwned(value Array*) Value:
    out Value = memory.zeroValue[Value]()
    out.kind = 5
    out.owned = true
    r Array** = cast.reinterpret[Array*](addrof out.value)
    *r = value
    ret out
..

# Inserts or replaces key and transfers value ownership into the object.
# @complexity O(N) lookup plus key-copy cost
# @ownership Consumes value, including on failure.
# @example
#   try object.set("name", json.stringBorrowed("Magma"))
Object.set(key str, value $Value) !void:
    try this.entries.set(key, value)
..

# Returns a non-owning value for key.
# @throws outOfBounds if key is absent
# @complexity O(N)
# @example
#   name := try object.get("name")
Object.get(key str) !Value:
    value := try this.entries.get(key)
    ret value.borrowed()
..

# Removes key and frees its owned value.
# @throws outOfBounds if key is absent
# @complexity O(N)
# @example
#   try object.delete("temporary")
Object.delete(key str) !void:
    try this.entries.delete(key)
..

# Removes key and transfers its value to the caller without freeing it.
# @throws outOfBounds if key is absent
# @ownership The caller becomes responsible for the returned value.
# @complexity O(N)
# @example
#   value := try object.take("payload")
Object.take(key str) !$Value:
    ret try this.entries.take(key)
..

# Returns the number of object members.
# @complexity O(1)
# @example
#   fields := object.count()
Object.count() u64:
    ret this.entries.count()
..

# Frees all keys, owned values, and object storage.
# @complexity O(N)
# @example
#   object.free()
destr Object.free() void:
    this.entries.free()
..

# Appends a value and transfers its ownership into the array.
# @complexity O(1) amortized, O(N) when storage grows
# @ownership Consumes value.
# @example
#   try items.append(json.numberInt(1))
Array.append(value $Value) !void:
    try this.values.pushRight(this.allocator, value)
..

# Returns the number of array elements.
# @complexity O(1)
# @example
#   length := items.count()
Array.count() u64:
    ret this.values.count()
..

# Returns a non-owning value at index.
# @throws invalidArgument if index is outside the array
# @complexity O(1)
# @example
#   first := try items.get(0)
Array.get(index u64) !Value:
    if index >= this.count():
        throw errors.invalidArgument("JSON array index out of bounds")
    ..
    values := this.values.view()
    value := values[index]
    ret value.borrowed()
..

# Frees all owned values and array storage.
# @complexity O(N)
# @example
#   items.free()
destr Array.free() void:
    this.values.free(this.allocator, valueCleanup)
..

writeEscaped(w writer.Writer, value str) !void:
    single := array u8[1]
    single[0] = 34
    try w.writeAll(strings.fromPtrNoCopy(slices.toPtr(single), 1))
    i u64 = 0
    bound := strings.countBytes(value)
    hex str = "0123456789abcdef"
    pair := array u8[2]
    pair[0] = 92
    escaped := array u8[6]
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
    ret (*bits & 0x7FF0000000000000) != 0x7FF0000000000000
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
        booleanPtr bool* = cast.reinterpret[bool](addrof valueCopy.value)
        try w.writeBool(*booleanPtr)
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
        if *object == none:
            throw errors.invalidArgument("JSON object pointer is null")
        ..
        try writeObject(w, *object, precision)
    elif valueCopy.kind == 5:
        array Array** = cast.reinterpret[Array*](addrof valueCopy.value)
        if *array == none:
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

# Serializes this value as compact JSON, using precision for fractional numbers.
# @complexity O(N) for serialized byte count
# @example
#   try value.write(output, 6)
Value.write(w writer.Writer, precision u64) !void:
    try writeValue(w, *this, precision)
..

# Serializes this object as compact JSON in insertion order.
# @complexity O(N) for serialized byte count
# @example
#   try object.write(output, 6)
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

# Serializes this array as compact JSON.
# @complexity O(N) for serialized byte count
# @example
#   try items.write(output, 6)
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
