mod fmt
# Deferred, typed string formatting for output streams and owned strings.

use "std:allocator" alc
use "std:cast" cast
use "std:errors" errors
use "std:io" io
use "std:memory" memory
use "std:strings" strings
use "std:writer" writer

const INITIAL_CAPACITY u64 = 8
const KIND_STRING u8 = 1
const KIND_UINT u8 = 2
const KIND_INT u8 = 3
const KIND_BOOL u8 = 4
const KIND_FLOAT u8 = 5

Part(
    value u128
    kind u8
)

FloatPart(
    value f64
    precision u64
)

stringPart(value str) Part:
    out Part
    out.kind = KIND_STRING
    payload str* = cast.reinterpret[str](addrof out.value)
    *payload = value
    ret out
..

uintPart(value u64) Part:
    out Part
    out.kind = KIND_UINT
    payload u64* = cast.reinterpret[u64](addrof out.value)
    *payload = value
    ret out
..

intPart(value i64) Part:
    out Part
    out.kind = KIND_INT
    payload i64* = cast.reinterpret[i64](addrof out.value)
    *payload = value
    ret out
..

boolPart(value bool) Part:
    out Part
    out.kind = KIND_BOOL
    payload bool* = cast.reinterpret[bool](addrof out.value)
    *payload = value
    ret out
..

floatPart(value f64, precision u64) Part:
    out Part
    out.kind = KIND_FLOAT
    payload FloatPart* = cast.reinterpret[FloatPart](addrof out.value)
    *payload = FloatPart(value=value, precision=precision)
    ret out
..

# A short-lived, deferred sequence of typed formatting operations. Strings are
# borrowed and must remain valid until the Format is consumed or freed.
pub Format(
    allocator alc.Allocator
    parts Part*
    count u64
    capacity u64
    failure error
)

failed(f Format*) bool:
    ret f.failure.nok()
..

remember(f Format*, failure error) void:
    if f.failure.ok():
        f.failure = failure
    ..
..

ensureCapacity(f Format*) bool:
    if failed(f) || f.count < f.capacity:
        ret f.failure.ok()
    ..

    maxU64 u64 = 0 - 1
    if f.capacity > maxU64 / 2:
        remember(f, errors.wouldOverflow("format part capacity overflow"))
        ret false
    ..

    newCapacity := f.capacity * 2
    resized Part*, resizeErr error = f.allocator.reallocT[Part](f.parts, newCapacity)
    if resizeErr.nok():
        remember(f, resizeErr)
        ret false
    ..
    f.parts = resized
    f.capacity = newCapacity
    ret true
..

append(f Format*, part Part) void:
    if ensureCapacity(f) == false:
        ret
    ..
    f.parts[f.count] = part
    f.count = f.count + 1
..

# Starts a format with one borrowed string and room for eight parts. Allocation
# failure is retained and reported by the terminal operation.
# @complexity O(1), excluding allocator cost
# @ownership initial is borrowed until the Format is consumed or freed.
# @example
#   format := fmt.str(a, "count: ").uint(42)
pub str(a alc.Allocator, initial str) $Format:
    parts Part*, allocErr error = a.allocT[Part](INITIAL_CAPACITY)
    result := Format(
        allocator=a,
        parts=parts,
        count=0,
        capacity=INITIAL_CAPACITY,
        failure=allocErr,
    )
    append(addrof result, stringPart(initial))
    ret result
..

# Appends a borrowed string part.
# @complexity Amortized O(1)
# @ownership value must remain valid until the Format is consumed or freed.
Format.str(value str) $Format:
    append(this, stringPart(value))
    ret *this
..

# Appends an unsigned decimal integer.
# @complexity Amortized O(1)
Format.uint(value u64) $Format:
    append(this, uintPart(value))
    ret *this
..

# Appends a signed decimal integer.
# @complexity Amortized O(1)
Format.int(value i64) $Format:
    append(this, intPart(value))
    ret *this
..

# Appends `true` or `false`.
# @complexity Amortized O(1)
Format.boolean(value bool) $Format:
    append(this, boolPart(value))
    ret *this
..

# Appends a floating-point value with precision digits after the decimal point.
# @complexity Amortized O(1) to append; O(precision) when rendered
Format.float(value f64, precision u64) $Format:
    append(this, floatPart(value, precision))
    ret *this
..

writeParts(f Format*, out writer.Writer) !u64:
    if failed(f):
        throw f.failure
    ..
    if f.count > f.capacity:
        throw errors.failure("invalid format part count")
    ..

    written u64 = 0
    i u64 = 0
    while i < f.count:
        part := f.parts[i]
        next u64 = 0
        if part.kind == KIND_STRING:
            payload str* = cast.reinterpret[str](addrof part.value)
            next = try out.writeAll(*payload)
        elif part.kind == KIND_UINT:
            payload u64* = cast.reinterpret[u64](addrof part.value)
            next = try out.writeUint64(*payload)
        elif part.kind == KIND_INT:
            payload i64* = cast.reinterpret[i64](addrof part.value)
            next = try out.writeInt64(*payload)
        elif part.kind == KIND_BOOL:
            payload bool* = cast.reinterpret[bool](addrof part.value)
            next = try out.writeBool(*payload)
        elif part.kind == KIND_FLOAT:
            payload FloatPart* = cast.reinterpret[FloatPart](addrof part.value)
            next = try out.writeFloat64(payload.value, payload.precision)
        else:
            throw errors.invalidType("unknown format part type")
        ..

        maxU64 u64 = 0 - 1
        if next > maxU64 - written:
            throw errors.wouldOverflow("formatted byte count overflow")
        ..
        written = written + next
        i = i + 1
    ..
    ret written
..

release(f Format*) void:
    if f.parts != none:
        f.allocator.free(f.parts)
        f.parts = none
    ..
    f.count = 0
    f.capacity = 0
..

# Writes the deferred format and consumes it. A construction error is reported
# before any bytes are written.
# @complexity O(P + B), for part count P and rendered byte count B
# @returns total bytes written
# @example
#   written := try format.writeTo(output)
destr Format.writeTo(out writer.Writer) !u64:
    written u64, writeErr error = writeParts(this, out)
    release(this)
    if writeErr.nok():
        throw writeErr
    ..
    ret written
..

# Writes the deferred format to standard output and consumes it.
# @complexity O(P + B)
# @returns total bytes written
# @example
#   try fmt.str(a, "count: ").uint(42).print()
destr Format.print() !u64:
    out := io.stdoutConst().toWriter()
    written u64, writeErr error = writeParts(this, out)
    release(this)
    if writeErr.nok():
        throw writeErr
    ..
    ret written
..

# Writes and consumes a deferred format on standard output.
# @complexity O(P + B)
# @example
#   try fmt.printf(fmt.str(a, "ready: ").boolean(true))
pub printf(format $Format) !void:
    try format.print()
..

countBytes(impl ptr, bytes str) !u64:
    ret strings.countBytes(bytes)
..

BufferSink(
    out u8*
    offset u64
)

writeBuffer(impl ptr, bytes str) !u64:
    sink BufferSink* = impl
    count := strings.countBytes(bytes)
    i u64 = 0
    while i < count:
        sink.out[sink.offset + i] = strings.byteAt(bytes, i)
        i = i + 1
    ..
    sink.offset = sink.offset + count
    ret count
..

# Renders the deferred format into a newly allocated string and consumes it.
# @complexity O(P + B)
# @ownership Release the returned string with a.
# @example
#   text := try format.toStr(a)
destr Format.toStr(a alc.Allocator) !$str:
    if failed(this):
        constructionErr := this.failure
        release(this)
        throw constructionErr
    ..
    counter := writer.new(none, countBytes)
    total u64, countErr error = writeParts(this, counter)
    if countErr.nok():
        release(this)
        throw countErr
    ..
    result $str, allocErr error = strings.alloc(a, total)
    if allocErr.nok():
        release(this)
        throw allocErr
    ..

    sink := BufferSink(out=strings.toPtr(result), offset=0)
    out := writer.new(addrof sink, writeBuffer)
    ignored u64, writeErr error = writeParts(this, out)
    release(this)
    if writeErr.nok():
        result.free(a)
        throw writeErr
    ..
    ret result
..

# Discards a format without rendering it.
# @complexity O(1), excluding allocator cost
# @example
#   format.free()
destr Format.free() void:
    release(this)
..
