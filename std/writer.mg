mod writer

use "strings.mg" strings
use "slices.mg"  slices
use "cast.mg"    cast

Writer(
    impl ptr,

    fn_write (ptr, str) !u64,
)

Writer.write(bytes str) !u64:
    ret try this.fn_write(this.impl, bytes)
..

Writer.writeLn(bytes str) !u64:
    written u64 = try this.fn_write(this.impl, bytes)
    written = written + try this.fn_write(this.impl, "\n")
    ret written
..

Writer.writeBool(b bool) !u64:
    if b == true:
        ret try this.fn_write(this.impl, "true")
    else:
        ret try this.fn_write(this.impl, "false")
    ..
..

digitToChar(i i16) u8:
    if i > 9:
        ret 0
    ..
    digits str = "0123456789"
    ret strings.byteAt(digits, i)
..

Writer.writeInt64(num i64) !u64:
    buf u8[20]
    n i64 = num
    idx u64 = 20
    isNeg bool = false

    if n == 0:
        ret try this.fn_write(this.impl, "0")
    ..

    if n < 0:
        isNeg = true
        n = 0 - n
    ..

    while n != 0:
        idx = idx - 1
        d i16 = cast.i64to16(n % 10)
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

    ret try this.fn_write(this.impl, toWrite)
..

Writer.writeUint64(num u64) !u64:
    buf u8[20]
    n u64 = num
    idx u64 = 20

    if n == 0:
        ret try this.fn_write(this.impl, "0")
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

    ret try this.fn_write(this.impl, toWrite)
..


Writer.writeFloat64(flt f64, precision u64) !u64:
    # bitcast and read bits as u64

    # TODO: fix not being able to assign to arg
    prec u64 = precision

    # TODO: fix issue where addrof arg returns ssa of arg instead of ptr
    fltCpy f64 = flt
    fltAddr ptr = addrof fltCpy
    addrAsU64 u64* = fltAddr
    bits u64 = addrAsU64[0]

    EXP_MASK  u64 = 0x7FF0000000000000
    FRAC_MASK u64 = 0x000FFFFFFFFFFFFF
    SIGN_MASK u64 = 0x8000000000000000

    exp  u64 = bits & EXP_MASK
    frac u64 = bits & FRAC_MASK
    sign bool = (bits & SIGN_MASK) != 0

    # is Nan
    if (exp == EXP_MASK) && (frac != 0):
        ret try this.fn_write(this.impl, "nan")
    ..

    # is infinitte
    if (exp == EXP_MASK) && (frac == 0):
        if sign:
            ret try this.fn_write(this.impl, "-inf")
        else:
            ret try this.fn_write(this.impl, "inf")
        ..
    ..

    # cap precision to keep buffer bounded
    if prec > 42:
        prec = 42
    ..

    buf u8[64]
    idx u64 = 64
    isNeg bool = false

    if fltCpy < 0.0:
        isNeg = true
        fltCpy = 0.0 - fltCpy
    ..

    intPart u64 = cast.ftou(fltCpy)
    fracPart f64 = fltCpy - cast.utof(intPart)

    scale f64 = 1.0
    i u64 = 0
    while i < prec:
        scale = scale * 10.0
        i = i + 1
    ..

    fracInt u64 = 0
    if prec > 0:
        fracInt = cast.ftou(fracPart * scale + 0.5)
    ..

    if (prec > 0) && (cast.utof(fracInt) >= scale):
        fracInt = fracInt - cast.ftou(scale)
        intPart = intPart + 1
    ..

    # TODO: fix multiple defs not allowed from different scope
    # (we could just reuse it?)
    d u64 = 0

    if prec > 0:
        i = 0
        while i < prec:
            idx = idx - 1
            d = fracInt % 10
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
            d = n % 10
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

    ret try this.fn_write(this.impl, toWrite)
..
