mod writer
# Type-erased byte output with complete-write and primitive formatting helpers.

use "std:strings" strings
use "std:slices"  slices
use "std:cast"    cast
use "std:errors"  errors

# Writer interface for emitting bytes and formatted values.
# @complexity O(1) wrapper calls; underlying writer decides cost.
# Mutable type-erased byte sink. The implementation pointer is borrowed and
# must remain alive and unmoved while this interface is used.
pub Writer(
    impl ptr,
    fn_write (ptr, str) !u64,
)

# Immutable-vtable writer variant. This representation is useful for global
# interfaces whose implementation and operation table never change.
pub Vtable(
    fn_write (ptr, str) !u64,
)

# Type-erased byte sink whose write callback receives immutable implementation state.
pub ConstWriter(
    impl ptr,
    vtable Vtable*,
)

# Attempts one write and returns the number of bytes accepted.
# @warning A successful call may write fewer bytes than requested.
# @complexity O(N), determined by the underlying sink
# @example
#   written := try output.write("data")
ConstWriter.write(bytes str) !u64:
    ret try this.vtable.fn_write(this.impl, bytes)
..

constWriterWriteRemaining(cw ConstWriter*, bytes str, firstWritten u64) !u64:
    total u64 = firstWritten
    bound u64 = strings.countBytes(bytes)
    base ptr = strings.toPtr(bytes)
    while total < bound:
        remaining u64 = bound - total
        next ptr = cast.utop(cast.ptou(base) + total)
        written u64 = try cw.write(strings.fromPtrNoCopy(next, remaining))
        if written > remaining:
            throw errors.failure("writer returned more bytes than requested")
        ..
        if written == 0:
            throw errors.failure("writer made no progress")
        ..
        total = total + written
    ..
    ret total
..

# Repeats writes until every byte is accepted or the sink fails to progress.
# @complexity O(N) plus underlying call overhead
# @example
#   try output.writeAll("complete payload")
ConstWriter.writeAll(bytes str) !u64:
    firstWritten u64 = try this.write(bytes)
    if firstWritten == strings.countBytes(bytes):
        ret firstWritten
    ..
    ret try constWriterWriteRemaining(this, bytes, firstWritten)
..

# Writes all bytes followed by a line-feed byte.
# @complexity O(N)
# @example
#   try output.writeLn("record")
ConstWriter.writeLn(bytes str) !u64:
    written u64 = try this.writeAll(bytes)
    written = written + try this.writeAll("\n")
    ret written
..

# Converts to the mutable Writer interface without changing the implementation.
# @ownership The returned interface borrows the same implementation.
# @complexity O(1)
# @example
#   writer := output.toWriter()
ConstWriter.toWriter() Writer:
    ret Writer(
        impl = this.impl
        fn_write = this.vtable.fn_write
    )
..

# Constructs a writer from borrowed implementation state and a write callback.
# @warning writeFunc must return no more than the requested byte count.
# @complexity O(1)
# @example
#   output := writer.new(context, writeBytes)
pub new(impl ptr, writeFunc (ptr, str) !u64) Writer:
    ret Writer(impl=impl, fn_write=writeFunc)
..

# Writes the provided bytes and returns the count written.
# @complexity O(N) for byte count.
# @param bytes string to write
# @returns number of bytes written
# @warning A successful call may write fewer bytes than requested.
# @example
#   written := try output.write("data")
Writer.write(bytes str) !u64:
    ret try this.fn_write(this.impl, bytes)
..

# Writes the complete byte string or returns an error if the adapter makes no
# progress or reports an invalid count.
# @complexity O(N) plus underlying call overhead
# @example
#   try output.writeAll("complete payload")
Writer.writeAll(bytes str) !u64:
    firstWritten u64 = try this.fn_write(this.impl, bytes)

    # Happy path, mean and lean
    if firstWritten == strings.countBytes(bytes):
        ret firstWritten
    ..

    total u64 = firstWritten
    bound u64 = strings.countBytes(bytes)
    base ptr = strings.toPtr(bytes)

    while total < bound:
        remaining u64 = bound - total
        next ptr = cast.utop(cast.ptou(base) + total)
        written u64 = try this.fn_write(this.impl, strings.fromPtrNoCopy(next, remaining))
        if written > remaining:
            throw errors.failure("writer returned more bytes than requested")
        ..
        if written == 0:
            throw errors.failure("writer made no progress")
        ..
        total = total + written
    ..
    ret total
..

# Writes the provided bytes followed by a newline.
# @complexity O(N) for byte count.
# @param bytes string to write
# @returns number of bytes written
# @example
#   try output.writeLn("record")
Writer.writeLn(bytes str) !u64:
    written u64 = try this.writeAll(bytes)
    written = written + try this.writeAll("\n")
    ret written
..

# Writes "true" or "false" based on the boolean value.
# @complexity O(1).
# @param b boolean value
# @returns number of bytes written
# @example
#   try output.writeBool(true)
Writer.writeBool(b bool) !u64:
    if b == true:
        ret try this.writeAll("true")
    else:
        ret try this.writeAll("false")
    ..
..

# Converts a digit 0-9 to its ASCII character.
# @complexity O(1).
digitToChar(i i16) u8:
    if i > 9:
        ret 0
    ..
    digits str = "0123456789"
    ret strings.byteAt(digits, i)
..

# Writes a signed 64-bit integer in decimal form.
# @complexity O(1) bounded by integer width.
# @param num integer value
# @returns number of bytes written
# @example
#   try output.writeInt64(-42)
Writer.writeInt64(num i64) !u64:
    buf := array u8[20]
    n u64 = cast.itou(num)
    idx u64 = 20
    isNeg bool = false

    if n == 0:
        ret try this.writeAll("0")
    ..

    if num < 0:
        isNeg = true
        # -(num + 1) is representable even for INT64_MIN.
        n = cast.itou(0 - (num + 1)) + 1
    ..

    while n != 0:
        idx = idx - 1
        d i16 = cast.u64to16(n % 10)
        buf[idx] = digitToChar(d)
        n = n / 10
    ..

    if isNeg:
        idx = idx - 1
        buf[idx] = strings.byteAt("-", 0)
    ..

    len u64 = 20 - idx
    bufPtr ptr = slices.toPtr(buf)
    writePtr ptr = cast.utop(cast.ptou(bufPtr) + idx)
    toWrite str = strings.fromPtrNoCopy(writePtr, len)

    ret try this.writeAll(toWrite)
..

# Writes an unsigned 64-bit integer in decimal form.
# @complexity O(1) bounded by integer width.
# @param num integer value
# @returns number of bytes written
# @example
#   try output.writeUint64(42)
Writer.writeUint64(num u64) !u64:
    buf := array u8[20]
    n u64 = num
    idx u64 = 20

    if n == 0:
        ret try this.writeAll("0")
    ..

    while n != 0:
        idx = idx - 1
        d i16 = cast.u64to16(n % 10)
        buf[idx] = digitToChar(d)
        n = n / 10
    ..

    len u64 = 20 - idx
    bufPtr ptr = slices.toPtr(buf)
    writePtr ptr = cast.utop(cast.ptou(bufPtr) + idx)
    toWrite str = strings.fromPtrNoCopy(writePtr, len)

    ret try this.writeAll(toWrite)
..


# Writes a floating point value with the provided precision.
# @complexity O(P) for precision digits.
# @param flt floating point value
# @param precision digits after decimal point
# @returns number of bytes written
# @warning Precision is capped at 42 digits; large finite values must fit u64's integer range.
# @example
#   try output.writeFloat64(3.14159, 2)
Writer.writeFloat64(flt f64, precision u64) !u64:
    # bitcast and read bits as u64

    fltAddr ptr = addrof flt
    addrAsU64 u64* = fltAddr
    bits u64 = *addrAsU64

    EXP_MASK  u64 = 0x7FF0000000000000
    FRAC_MASK u64 = 0x000FFFFFFFFFFFFF
    SIGN_MASK u64 = 0x8000000000000000

    exp  u64 = bits & EXP_MASK
    frac u64 = bits & FRAC_MASK
    sign bool = (bits & SIGN_MASK) != 0

    # is Nan
    if (exp == EXP_MASK) && (frac != 0):
        ret try this.writeAll("nan")
    ..

    # is infinitte
    if (exp == EXP_MASK) && (frac == 0):
        if sign:
            ret try this.writeAll("-inf")
        else:
            ret try this.writeAll("inf")
        ..
    ..

    # cap precision to keep buffer bounded
    if precision > 42:
        precision = 42
    ..

    buf := array u8[64]
    idx u64 = 64
    isNeg bool = false
    fltCpy f64 = flt

    if fltCpy < 0.0:
        isNeg = true
        fltCpy = 0.0 - fltCpy
    ..

    intPart u64 = cast.ftou(fltCpy)
    fracPart f64 = fltCpy - cast.utof(intPart)

    scale f64 = 1.0
    i u64 = 0
    while i < precision:
        scale = scale * 10.0
        i = i + 1
    ..

    fracInt u64 = 0
    if precision > 0:
        fracInt = cast.ftou(fracPart * scale + 0.5)
    ..

    if (precision > 0) && (cast.utof(fracInt) >= scale):
        fracInt = fracInt - cast.ftou(scale)
        intPart = intPart + 1
    ..

    if precision > 0:
        i = 0
        while i < precision:
            idx = idx - 1
            d u64 = fracInt % 10
            buf[idx] = digitToChar(cast.u64to16(d))
            fracInt = fracInt / 10
            i = i + 1
        ..

        idx = idx - 1
        buf[idx] = strings.byteAt(".", 0)
    ..

    if intPart == 0:
        idx = idx - 1
        buf[idx] = strings.byteAt("0", 0)
    else:
        n u64 = intPart
        while n != 0:
            idx = idx - 1
            d u64 = n % 10
            buf[idx] = digitToChar(cast.u64to16(d))
            n = n / 10
        ..
    ..

    if isNeg:
        idx = idx - 1
        buf[idx] = strings.byteAt("-", 0)
    ..

    len u64 = 64 - idx
    bufPtr ptr = slices.toPtr(buf)
    writePtr ptr = cast.utop(cast.ptou(bufPtr) + idx)
    toWrite str = strings.fromPtrNoCopy(writePtr, len)

    ret try this.writeAll(toWrite)
..
