mod json
# Parsing, construction, lookup, ownership, and serialization of JSON values.

use "std:allocator"  alc
use "std:array"      arr
use "std:builder"    builder
use "std:cast"       cast
use "std:errors"     errors
use "std:footgun"    footgun
use "std:linear_map" linear_map
use "std:slices"     slices
use "std:strings"    strings
use "std:writer"     writer
use "std:memory"     memory
use "std:utf8"       utf8

pub const KIND_NULL u8 = 0
pub const KIND_BOOL u8 = 1
pub const KIND_FLOAT u8 = 2
pub const KIND_STRING u8 = 3
pub const KIND_OBJECT u8 = 4
pub const KIND_ARRAY u8 = 5

# JSON value. Payloads are stored in raw u128 storage and reinterpreted based
# on the kind tag. This keeps Value independent of its recursive payload types.
pub Value(
    value u128
    kind u8
    allocator alc.Allocator
)

# Borrowed, copyable view of a JSON object. Pointer storage is private to this
# module; public APIs pass Object by value.
pub Object(
    data ObjectData*
)

pub proto Serializable(
    toJson() !$Value
)

ObjectData(
    entries linear_map.LinearMap[Value]
)

# Borrowed, copyable view of a JSON array. Pointer storage is private to this
# module; public APIs pass Array by value.
pub Array(
    data ArrayData*
)

ArrayData(
    allocator alc.Allocator
    values arr.Array[Value]
)

# Validates that this value is JSON null.
# @throws invalidType if this value has another kind
# @complexity O(1)
# @example
#   try value.asNull()
Value.asNull() !void:
    if this.kind != 0:
        throw errors.invalidType("json value is not null")
    ..
    ret
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

# Returns a borrowed object view.
# @throws invalidType if this value is not an object
# @ownership The returned view borrows storage from this value.
# @complexity O(1)
# @example
#   object := try value.asObject()
Value.asObject() !Object:
    if this.kind != 4:
        throw errors.invalidType("json value is not object")
    ..
    r Object* = cast.reinterpret[Object](addrof this.value)
    ret *r
..

# Returns a borrowed array view.
# @throws invalidType if this value is not an array
# @ownership The returned view borrows storage from this value.
# @complexity O(1)
# @example
#   items := try value.asArray()
Value.asArray() !Array:
    if this.kind != 5:
        throw errors.invalidType("json value is not array")
    ..
    r Array* = cast.reinterpret[Array](addrof this.value)
    ret *r
..

releaseValue(val Value) void:
    if val.kind == 3:
        value str* = cast.reinterpret[str](addrof val.value)
        val.allocator.free(strings.toPtr(*value))
    elif val.kind == 4:
        value Object* = cast.reinterpret[Object](addrof val.value)
        if value.data != none:
            unsafe:
                value.data.entries.free()
                val.allocator.free(value.data)
            ..
        ..
    elif val.kind == 5:
        value Array* = cast.reinterpret[Array](addrof val.value)
        if value.data != none:
            unsafe:
                value.data.values.free(value.data.allocator, arrayValueCleanup)
                val.allocator.free(value.data)
            ..
        ..
    ..
..

valueCleanup(val $Value) void:
    releaseValue(val)
    footgun.drop[Value](move val)
..

# Releases an owned JSON value and all of its descendants.
# @complexity O(N) for arrays and objects
# @example
#   defer value.free()
destr Value.free() void:
    releaseValue(*this)
..

# Creates an owned empty JSON object.
pub object() !$Value:
    a := ctx.procAlloc
    data ObjectData* = try a.allocT[ObjectData](1)
    onerror a.free(data)
    entries := try linear_map.new[Value](valueCleanup)
    unsafe:
        data.entries = move entries
    ..
    out := Value(value=0, kind=4, allocator=a)
    objectView := Object(data=data)
    payload Object* = cast.reinterpret[Object](addrof out.value)
    *payload = objectView
    ret move out
..

# Creates an owned empty JSON array.
pub array() !$Value:
    a := ctx.procAlloc
    data ArrayData* = try a.allocT[ArrayData](1)
    onerror a.free(data)
    values := try arr.new[Value](a)
    unsafe:
        data.allocator = a
        data.values = move values
    ..
    out := Value(value=0, kind=5, allocator=a)
    arrayView := Array(data=data)
    payload Array* = cast.reinterpret[Array](addrof out.value)
    *payload = arrayView
    ret move out
..

# Creates a JSON null value.
# @complexity O(1)
# @example
#   value := json.null()
pub null() $Value:
    ret memory.zeroValue[Value]()
..

# Creates a JSON boolean value.
# @complexity O(1)
# @example
#   value := json.bool(true)
pub bool(value bool) $Value:
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
pub numberFloat(value f64) $Value:
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
pub numberInt(value i64) $Value:
    out Value = memory.zeroValue[Value]()
    out.kind = 6
    r i64* = cast.reinterpret[i64](addrof out.value)
    *r = value
    ret out
..

stringOwned(value $str) $Value:
    a := ctx.procAlloc
    out Value = memory.zeroValue[Value]()
    out.kind = 3
    out.allocator = a
    r str* = cast.reinterpret[str](addrof out.value)
    *r = move value
    ret out
..

# Copies text into an owned JSON string.
# @complexity O(N) for the string byte length
# @ownership The returned value owns its copy.
# @example
#   value := try json.string(input)
pub string(value str) !$Value:
    a := ctx.procAlloc
    owned str = try strings.copy(value)
    ret stringOwned(move owned)
..

# Inserts or replaces key and transfers value ownership into the object.
# @complexity O(N) lookup plus key-copy cost
# @ownership Consumes value, including on failure.
# @example
#   try object.set("name", try json.string("Magma"))
Object.set(key str, value $Value) !void:
    try this.data.entries.set(key, move value)
..

# Returns a non-owning value for key.
# @throws outOfBounds if key is absent
# @complexity O(N)
# @example
#   name := try object.get("name")
Object.get(key str) !Value:
    ret try this.data.entries.get(key)
..

# Removes key and frees its owned value.
# @throws outOfBounds if key is absent
# @complexity O(N)
# @example
#   try object.delete("temporary")
Object.delete(key str) !void:
    try this.data.entries.delete(key)
..

# Removes key and transfers its value to the caller without freeing it.
# @throws outOfBounds if key is absent
# @ownership The caller becomes responsible for the returned value.
# @complexity O(N)
# @example
#   value := try object.take("payload")
Object.take(key str) !$Value:
    ret try this.data.entries.take(key)
..

# Returns the number of object members.
# @complexity O(1)
# @example
#   fields := object.count()
Object.count() u64:
    ret this.data.entries.count()
..

# Copies text and inserts it under key.
Object.setString(key str, value str) !void:
    text := try string(value)
    try this.set(key, move text)
..

Object.setInt(key str, value i64) !void:
    try this.set(key, numberInt(value))
..

Object.setBool(key str, value bool) !void:
    try this.set(key, bool(value))
..

# Appends a value and transfers its ownership into the array.
# @complexity O(1) amortized, O(N) when storage grows
# @ownership Consumes value.
# @example
#   try items.append(json.numberInt(1))
Array.append(value $Value) !void:
    index u64, expandError error = this.data.values.expandRight(this.data.allocator)
    if expandError.nok():
        valueCleanup(move value)
        throw expandError
    ..
    try this.data.values.set(this.data.allocator, index, move value, arrayValueCleanup)
..

arrayValueCleanup(a alc.Allocator, val $Value) void:
    valueCleanup(move val)
..

# Returns the number of array elements.
# @complexity O(1)
# @example
#   length := items.count()
Array.count() u64:
    ret this.data.values.count()
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
    values := this.data.values.view()
    bounded index < values.count():
        ret values[index]
    ..
..

# Object operations forwarded through an owned or borrowed Value.
Value.get(key str) !Value:
    view := try this.asObject()
    ret try view.get(key)
..

Value.set(key str, value $Value) !void:
    view Object, viewError error = this.asObject()
    if viewError.nok():
        valueCleanup(move value)
        throw viewError
    ..
    try view.set(key, move value)
..

Value.setString(key str, value str) !void:
    view := try this.asObject()
    try view.setString(key, value)
..

Value.setInt(key str, value i64) !void:
    view := try this.asObject()
    try view.setInt(key, value)
..

Value.setBool(key str, value bool) !void:
    view := try this.asObject()
    try view.setBool(key, value)
..

# Returns a borrowed array element. Named `at` because Magma does not overload
# Value.get for both string keys and integer indices.
Value.at(index u64) !Value:
    view := try this.asArray()
    ret try view.get(index)
..

Value.append(value $Value) !void:
    view Array, viewError error = this.asArray()
    if viewError.nok():
        valueCleanup(move value)
        throw viewError
    ..
    try view.append(move value)
..

Value.count() !u64:
    if this.kind == 4:
        view := try this.asObject()
        ret view.count()
    elif this.kind == 5:
        view := try this.asArray()
        ret view.count()
    ..
    throw errors.invalidType("json value is not a container")
..

const MAX_PARSE_DEPTH u64 = 128

Parser(
    source str
    index u64
)

Parser.count() u64:
    ret this.source.countBytes()
..

Parser.has(count u64) bool:
    size := this.count()
    ret this.index <= size && count <= size - this.index
..

Parser.current() u8:
    ret strings.byteAt(this.source, this.index)
..

isJsonWhitespace(byte u8) bool:
    ret byte == 32 || byte == 9 || byte == 10 || byte == 13
..

Parser.skipWhitespace() void:
    loop this.index < this.count() && isJsonWhitespace(this.current()):
        this.index = this.index + 1
    ..
..

Parser.view(start u64, end u64) str:
    # SAFETY: parser-produced offsets are bounded by source.countBytes().
    unsafe:
        startPtr := cast.utop(cast.ptou(strings.toPtr(this.source)) + start)
        ret strings.fromPtrNoCopy(startPtr, end - start)
    ..
..

hexValue(byte u8) !u32:
    if byte >= 48 && byte <= 57:
        ret cast.u64to32(cast.u8to64(byte - 48))
    elif byte >= 65 && byte <= 70:
        ret cast.u64to32(cast.u8to64(byte - 65) + 10)
    elif byte >= 97 && byte <= 102:
        ret cast.u64to32(cast.u8to64(byte - 97) + 10)
    ..
    throw errors.invalidArgument("invalid hexadecimal digit in JSON escape")
..

Parser.parseHexUnit() !u32:
    if this.has(4) == false:
        throw errors.invalidArgument("truncated Unicode escape in JSON string")
    ..
    value u32 = 0
    for offset u64 = 0 to 4:
        value = (value << 4) | try hexValue(strings.byteAt(this.source, this.index + offset))
    ..
    this.index = this.index + 4
    ret value
..

Parser.appendScalar(output builder.Builder*, scalar u32) !void:
    bytes := array u8[4]
    view := slices.fromPtr(slices.toPtr(bytes), 4)
    width := try utf8.encode(scalar, view)
    encoded := strings.fromPtrNoCopy(slices.toPtr(bytes), width)
    try output.appendCopy(encoded)
..

Parser.parseString() !$str:
    if this.has(1) == false || this.current() != 34:
        throw errors.invalidArgument("expected JSON string")
    ..
    this.index = this.index + 1
    segmentStart := this.index
    output := try builder.new()
    defer output.free()

    loop this.index < this.count():
        byte := this.current()
        if byte == 34:
            if this.index > segmentStart:
                try output.appendBorrowed(this.view(segmentStart, this.index))
            ..
            this.index = this.index + 1
            result := try output.build()
            if utf8.validate(result) == false:
                result.free(ctx.procAlloc)
                throw errors.invalidArgument("invalid UTF-8 in JSON string")
            ..
            ret move result
        elif byte < 32:
            throw errors.invalidArgument("unescaped control character in JSON string")
        elif byte != 92:
            this.index = this.index + 1
            continue
        ..

        if this.index > segmentStart:
            try output.appendBorrowed(this.view(segmentStart, this.index))
        ..
        this.index = this.index + 1
        if this.has(1) == false:
            throw errors.invalidArgument("truncated escape in JSON string")
        ..
        escape := this.current()
        this.index = this.index + 1
        if escape == 34:
            try output.appendBorrowed("\"")
        elif escape == 92:
            try output.appendBorrowed("\\")
        elif escape == 47:
            try output.appendBorrowed("/")
        elif escape == 98:
            try output.appendBorrowed("\b")
        elif escape == 102:
            try output.appendBorrowed("\f")
        elif escape == 110:
            try output.appendBorrowed("\n")
        elif escape == 114:
            try output.appendBorrowed("\r")
        elif escape == 116:
            try output.appendBorrowed("\t")
        elif escape == 117:
            scalar := try this.parseHexUnit()
            if scalar >= 55296 && scalar <= 56319:
                if this.has(6) == false || this.current() != 92 || strings.byteAt(this.source, this.index + 1) != 117:
                    throw errors.invalidArgument("high surrogate is missing its low surrogate")
                ..
                this.index = this.index + 2
                low := try this.parseHexUnit()
                if low < 56320 || low > 57343:
                    throw errors.invalidArgument("invalid low surrogate in JSON string")
                ..
                scalar = 65536 + ((scalar - 55296) << 10) + (low - 56320)
            elif scalar >= 56320 && scalar <= 57343:
                throw errors.invalidArgument("unexpected low surrogate in JSON string")
            ..
            try this.appendScalar(addrof output, scalar)
        else:
            throw errors.invalidArgument("invalid escape in JSON string")
        ..
        segmentStart = this.index
    ..
    throw errors.invalidArgument("unterminated JSON string")
..

Parser.consumeLiteral(expected str) !void:
    size := expected.countBytes()
    if this.has(size) == false:
        throw errors.invalidArgument("truncated JSON literal")
    ..
    for offset u64 = 0 to size:
        if strings.byteAt(this.source, this.index + offset) != strings.byteAt(expected, offset):
            throw errors.invalidArgument("invalid JSON literal")
        ..
    ..
    this.index = this.index + size
..

isDigit(byte u8) bool:
    ret byte >= 48 && byte <= 57
..

Parser.parseInteger(start u64, end u64, negative bool) Value:
    position := start
    if negative:
        position = position + 1
    ..
    limit u64 = 9223372036854775807
    if negative:
        limit = 9223372036854775808
    ..
    magnitude u64 = 0
    loop position < end:
        digit := cast.u8to64(strings.byteAt(this.source, position) - 48)
        if magnitude > limit / 10 || (magnitude == limit / 10 && digit > limit % 10):
            ret memory.zeroValue[Value]()
        ..
        magnitude = magnitude * 10 + digit
        position = position + 1
    ..
    if negative:
        if magnitude == 9223372036854775808:
            ret numberInt(cast.utoi(magnitude))
        ..
        ret numberInt(0 - cast.utoi(magnitude))
    ..
    ret numberInt(cast.utoi(magnitude))
..

power10(value f64, exponent i64) f64:
    result := value
    remaining := exponent
    if remaining > 0:
        loop remaining > 0:
            result = result * 10.0
            remaining = remaining - 1
        ..
    else:
        loop remaining < 0:
            result = result / 10.0
            remaining = remaining + 1
        ..
    ..
    ret result
..

Parser.parseNumber() !Value:
    start := this.index
    negative bool = false
    if this.current() == 45:
        negative = true
        this.index = this.index + 1
        if this.has(1) == false:
            throw errors.invalidArgument("truncated JSON number")
        ..
    ..

    if this.current() == 48:
        this.index = this.index + 1
        if this.index < this.count() && isDigit(this.current()):
            throw errors.invalidArgument("leading zero in JSON number")
        ..
    elif this.current() >= 49 && this.current() <= 57:
        loop this.index < this.count() && isDigit(this.current()):
            this.index = this.index + 1
        ..
    else:
        throw errors.invalidArgument("invalid integer part in JSON number")
    ..

    integerEnd := this.index
    fractionDigits u64 = 0
    hasFraction bool = false
    if this.index < this.count() && this.current() == 46:
        hasFraction = true
        this.index = this.index + 1
        fractionStart := this.index
        loop this.index < this.count() && isDigit(this.current()):
            this.index = this.index + 1
        ..
        fractionDigits = this.index - fractionStart
        if fractionDigits == 0:
            throw errors.invalidArgument("JSON fraction requires a digit")
        ..
    ..

    explicitExponent i64 = 0
    exponentNegative bool = false
    hasExponent bool = false
    if this.index < this.count() && (this.current() == 101 || this.current() == 69):
        hasExponent = true
        this.index = this.index + 1
        if this.index < this.count() && (this.current() == 43 || this.current() == 45):
            exponentNegative = this.current() == 45
            this.index = this.index + 1
        ..
        exponentStart := this.index
        loop this.index < this.count() && isDigit(this.current()):
            digit := cast.u8to64(this.current() - 48)
            if explicitExponent < 10000:
                explicitExponent = explicitExponent * 10 + cast.utoi(digit)
                if explicitExponent > 10000:
                    explicitExponent = 10000
                ..
            ..
            this.index = this.index + 1
        ..
        if this.index == exponentStart:
            throw errors.invalidArgument("JSON exponent requires a digit")
        ..
        if exponentNegative:
            explicitExponent = 0 - explicitExponent
        ..
    ..

    if hasFraction == false && hasExponent == false:
        integer := this.parseInteger(start, integerEnd, negative)
        if integer.kind == 6:
            ret integer
        ..
    ..

    mantissa f64 = 0.0
    significantTotal u64 = 0
    kept u64 = 0
    leadingDigits u64 = 0
    leadingCount u64 = 0
    nonzeroAfterLeading bool = false
    seenNonzero bool = false
    position := start
    if negative:
        position = position + 1
    ..
    numberEnd := this.index
    loop position < numberEnd:
        byte := strings.byteAt(this.source, position)
        if byte == 101 || byte == 69:
            break
        elif byte == 46:
            position = position + 1
            continue
        ..
        digit := cast.u8to64(byte - 48)
        if digit != 0:
            seenNonzero = true
        ..
        if seenNonzero:
            significantTotal = significantTotal + 1
            if leadingCount < 20:
                leadingDigits = leadingDigits * 10 + digit
                leadingCount = leadingCount + 1
            elif digit != 0:
                nonzeroAfterLeading = true
            ..
            if kept < 19:
                mantissa = mantissa * 10.0 + cast.utof(digit)
                kept = kept + 1
            ..
        ..
        position = position + 1
    ..
    if seenNonzero == false:
        if negative:
            ret numberFloat(0.0 - 0.0)
        ..
        ret numberFloat(0.0)
    ..

    # The largest finite f64 begins 1.7976931348623157081e308. Checking the
    # decimal order and its first 20 significant digits prevents conversion
    # rounding from silently turning a mathematically out-of-range number into
    # the largest finite value.
    decimalOrder := explicitExponent - cast.utoi(fractionDigits)
    decimalOrder = decimalOrder + cast.utoi(significantTotal) - 1
    if decimalOrder > 308:
        throw errors.invalidArgument("JSON number is outside f64 range")
    elif decimalOrder == 308:
        loop leadingCount < 20:
            leadingDigits = leadingDigits * 10
            leadingCount = leadingCount + 1
        ..
        if leadingDigits > 17976931348623157081 || (leadingDigits == 17976931348623157081 && nonzeroAfterLeading):
            throw errors.invalidArgument("JSON number is outside f64 range")
        ..
    ..

    decimalExponent := explicitExponent - cast.utoi(fractionDigits)
    skipped := significantTotal - kept
    if skipped > 10000:
        # Explicit exponents are clamped to 10,000, so 10,001 is sufficient
        # to preserve the fact that the effective exponent remains positive.
        decimalExponent = decimalExponent + 10001
    else:
        decimalExponent = decimalExponent + cast.utoi(skipped)
    ..
    if decimalExponent > 400:
        throw errors.invalidArgument("JSON number is outside f64 range")
    elif decimalExponent < -400:
        mantissa = 0.0
    else:
        mantissa = power10(mantissa, decimalExponent)
    ..
    if negative:
        mantissa = 0.0 - mantissa
    ..
    if finite(mantissa) == false:
        throw errors.invalidArgument("JSON number is outside f64 range")
    ..
    ret numberFloat(mantissa)
..

setParsedObjectValue(view Object, key str, value $Value) !bool:
    try view.set(key, move value)
    ret true
..

Parser.parseObject(depth u64) !$Value:
    a := ctx.procAlloc
    this.index = this.index + 1
    out := try object()
    onerror valueCleanup(move out)
    objectView := try out.asObject()
    this.skipWhitespace()
    if this.index < this.count() && this.current() == 125:
        this.index = this.index + 1
        ret move out
    ..
    loop true:
        if this.index >= this.count() || this.current() != 34:
            throw errors.invalidArgument("JSON object key must be a string")
        ..
        key := try this.parseString()
        this.skipWhitespace()
        if this.index >= this.count() || this.current() != 58:
            key.free(a)
            throw errors.invalidArgument("expected colon after JSON object key")
        ..
        this.index = this.index + 1
        value $Value, parseError error = this.parseValue(depth + 1)
        if parseError.nok():
            key.free(a)
            throw parseError
        ..
        inserted bool, setError error = setParsedObjectValue(objectView, key, move value)
        key.free(a)
        if setError.nok():
            throw setError
        ..
        this.skipWhitespace()
        if this.index >= this.count():
            throw errors.invalidArgument("unterminated JSON object")
        elif this.current() == 125:
            this.index = this.index + 1
            ret move out
        elif this.current() != 44:
            throw errors.invalidArgument("expected comma or closing brace in JSON object")
        ..
        this.index = this.index + 1
        this.skipWhitespace()
        if this.index < this.count() && this.current() == 125:
            throw errors.invalidArgument("trailing comma in JSON object")
        ..
    ..
..

Parser.parseArray(depth u64) !$Value:
    this.index = this.index + 1
    out := try array()
    onerror valueCleanup(move out)
    arrayView := try out.asArray()
    this.skipWhitespace()
    if this.index < this.count() && this.current() == 93:
        this.index = this.index + 1
        ret move out
    ..
    loop true:
        value := try this.parseValue(depth + 1)
        try arrayView.append(move value)
        this.skipWhitespace()
        if this.index >= this.count():
            throw errors.invalidArgument("unterminated JSON array")
        elif this.current() == 93:
            this.index = this.index + 1
            ret move out
        elif this.current() != 44:
            throw errors.invalidArgument("expected comma or closing bracket in JSON array")
        ..
        this.index = this.index + 1
        this.skipWhitespace()
        if this.index < this.count() && this.current() == 93:
            throw errors.invalidArgument("trailing comma in JSON array")
        ..
    ..
..

Parser.parseValue(depth u64) !$Value:
    this.skipWhitespace()
    if this.index >= this.count():
        throw errors.invalidArgument("expected JSON value")
    ..
    byte := this.current()
    if byte == 110:
        try this.consumeLiteral("null")
        ret null()
    elif byte == 116:
        try this.consumeLiteral("true")
        ret bool(true)
    elif byte == 102:
        try this.consumeLiteral("false")
        ret bool(false)
    elif byte == 34:
        value := try this.parseString()
        ret stringOwned(move value)
    elif byte == 123:
        if depth >= MAX_PARSE_DEPTH:
            throw errors.invalidArgument("JSON nesting limit exceeded")
        ..
        ret try this.parseObject(depth)
    elif byte == 91:
        if depth >= MAX_PARSE_DEPTH:
            throw errors.invalidArgument("JSON nesting limit exceeded")
        ..
        ret try this.parseArray(depth)
    elif byte == 45 || isDigit(byte):
        ret try this.parseNumber()
    ..
    throw errors.invalidArgument("unexpected character while parsing JSON value")
..

# Parses one complete JSON text into an owned value.
# Duplicate object keys use last-value-wins semantics.
# @throws invalidArgument for malformed JSON or nesting deeper than 128 containers
# @ownership Release the returned value with Value.free.
# @complexity O(N), excluding linear object-key lookup
pub parse(source str) !$Value:
    parser := Parser(source=source, index=0)
    value := try parser.parseValue(0)
    parser.skipWhitespace()
    if parser.index != source.countBytes():
        value.free()
        throw errors.invalidArgument("trailing content after JSON value")
    ..
    ret move value
..

writeEscaped(w writer.Writer, value str) !void:
    single := array u8[1]
    single[0] = 34
    try w.writeAll(strings.fromPtrNoCopy(slices.toPtr(single), 1))
    i u64 = 0
    bound := value.countBytes()
    hex str = "0123456789abcdef"
    pair := array u8[2]
    pair[0] = 92
    escaped := array u8[6]
    escaped[0] = 92
    escaped[1] = 117
    escaped[2] = 48
    escaped[3] = 48

    loop i < bound:
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
            escaped[4] = strings.byteAt(hex, byte >> 4)
            escaped[5] = strings.byteAt(hex, byte & 15)
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

writeObject(w writer.Writer, value Object, precision u64) !void:
    try value.write(w, precision)
..

writeArray(w writer.Writer, value Array, precision u64) !void:
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
        text str* = cast.reinterpret[str](addrof valueCopy.value)
        try writeEscaped(w, *text)
    elif valueCopy.kind == 4:
        objectView Object* = cast.reinterpret[Object](addrof valueCopy.value)
        if objectView.data == none:
            throw errors.invalidArgument("JSON object pointer is null")
        ..
        try writeObject(w, *objectView, precision)
    elif valueCopy.kind == 5:
        arrayView Array* = cast.reinterpret[Array](addrof valueCopy.value)
        if arrayView.data == none:
            throw errors.invalidArgument("JSON array pointer is null")
        ..
        try writeArray(w, *arrayView, precision)
    elif valueCopy.kind == 6:
        intNumber i64* = cast.reinterpret[i64](addrof valueCopy.value)
        try w.writeInt64(*intNumber)
    else:
        throw errors.invalidArgument("invalid JSON value kind")
    ..
..

# Serializes this value as compact JSON with six fractional digits.
# @complexity O(N) for serialized byte count
# @example
#   try value.write(output)
Value.write(w writer.Writer) !void:
    try writeValue(w, *this, 6)
..

# Serializes this value with an explicit fractional precision.
Value.writeWithPrecision(w writer.Writer, precision u64) !void:
    try writeValue(w, *this, precision)
..

# Serializes this object as compact JSON in insertion order.
# @complexity O(N) for serialized byte count
# @example
#   try object.write(output, 6)
Object.write(w writer.Writer, precision u64) !void:
    try w.writeAll("{")
    keys := this.data.entries.keysView()
    values := this.data.entries.valuesView()
    for i u64 = 0 to this.count():
        bounded i < keys.count(), i < values.count():
            if i != 0:
                try w.writeAll(",")
            ..
            try writeEscaped(w, keys[i])
            try w.writeAll(":")
            try writeValue(w, values[i], precision)
        ..
    ..
    try w.writeAll("}")
..

# Serializes this array as compact JSON.
# @complexity O(N) for serialized byte count
# @example
#   try items.write(output, 6)
Array.write(w writer.Writer, precision u64) !void:
    try w.writeAll("[")
    values := this.data.values.view()
    for i u64 = 0 to this.count():
        bounded i < values.count():
            if i != 0:
                try w.writeAll(",")
            ..
            try writeValue(w, values[i], precision)
        ..
    ..
    try w.writeAll("]")
..
