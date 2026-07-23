mod cast
# Explicit pointer, integer, and floating-point conversions.
# @warning Narrowing conversions discard bits when the value does not fit.

# Reinterprets an untyped pointer as a pointer to T without changing its address.
# @complexity O(1)
# @safety x must be correctly aligned and reference a valid T before dereferencing.
# @example
#   typed := cast.reinterpret[Header](raw)
pub reinterpret[T](x ptr) T*:
    ret x
..

# Casts a pointer to u64.
# @complexity O(1).
# @example
#   address := cast.ptou(pointer)
pub ptou(x ptr) u64:
    llvm "%x0 = ptrtoint ptr %x to i64\n"
    llvm "ret i64 %x0\n"
..

# Casts a u64 to pointer.
# @complexity O(1).
# @safety The integer must represent a valid pointer before dereferencing.
# @example
#   pointer := cast.utop(address)
pub utop(x u64) ptr:
    llvm "%x0 = inttoptr i64 %x to ptr\n"
    llvm "ret ptr %x0\n"
..

# Casts i64 to u64.
# @complexity O(1).
# @example
#   bits := cast.itou(value)
pub itou(x i64) u64:
    llvm "ret i64 %x\n"
..

# Casts u64 to i64.
# @complexity O(1).
# @example
#   signed := cast.utoi(bits)
pub utoi(x u64) i64:
    llvm "ret i64 %x\n"
..

# Zero-extends u64 to u128.
# @complexity O(1).
# @example
#   wide := cast.u64to128(value)
pub u64to128(x u64) u128:
    llvm "%c = zext i64 %x to i128\n"
    llvm "ret i128 %c\n"
..

# Truncates u128 to u64.
# @warning higher bits are discarded on overflow.
# @complexity O(1).
# @example
#   narrow := cast.u128to64(wide)
pub u128to64(x u128) u64:
    llvm "%c = trunc i128 %x to i64\n"
    llvm "ret i64 %c\n"
..

# Sign-extends i64 to i128.
# @complexity O(1).
# @example
#   wide := cast.i64to128(value)
pub i64to128(x i64) i128:
    llvm "%c = sext i64 %x to i128\n"
    llvm "ret i128 %c\n"
..

# Truncates i128 to i64.
# @warning higher bits are discarded on overflow.
# @complexity O(1).
# @example
#   narrow := cast.i128to64(wide)
pub i128to64(x i128) i64:
    llvm "%c = trunc i128 %x to i64\n"
    llvm "ret i64 %c\n"
..

# Converts signed i64 to f64.
# @complexity O(1).
# @warning Large integers may be rounded because f64 cannot represent every i64 exactly.
# @example
#   decimal := cast.itof(value)
pub itof(x i64) f64:
    llvm "%c = sitofp i64 %x to double\n"
    llvm "ret double %c\n"
..

# Converts unsigned u64 to f64.
# @complexity O(1).
# @warning Large integers may be rounded because f64 cannot represent every u64 exactly.
# @example
#   decimal := cast.utof(value)
pub utof(x u64) f64:
    llvm "%c = uitofp i64 %x to double\n"
    llvm "ret double %c\n"
..

# Converts f64 to signed i64.
# @complexity O(1).
# @warning Fractional values are truncated and out-of-range conversion is target-dependent.
# @example
#   integer := cast.ftoi(value)
pub ftoi(x f64) i64:
    llvm "%c = fptosi double %x to i64\n"
    llvm "ret i64 %c\n"
..

# Converts f64 to unsigned u64.
# @complexity O(1).
# @warning Fractional values are truncated and negative or out-of-range conversion is target-dependent.
# @example
#   integer := cast.ftou(value)
pub ftou(x f64) u64:
    llvm "%c = fptoui double %x to i64\n"
    llvm "ret i64 %c\n"
..

# Sign-extends i32 to i64.
# @complexity O(1).
# @example
#   wide := cast.i32to64(value)
pub i32to64(x i32) i64:
    llvm "%c = sext i32 %x to i64\n"
    llvm "ret i64 %c\n"
..

# Sign-extends i16 to i64.
# @complexity O(1).
# @example
#   wide := cast.i16to64(value)
pub i16to64(x i16) i64:
    llvm "%c = sext i16 %x to i64\n"
    llvm "ret i64 %c\n"
..

# Sign-extends i8 to i64.
# @complexity O(1).
# @example
#   wide := cast.i8to64(value)
pub i8to64(x i8) i64:
    llvm "%c = sext i8 %x to i64\n"
    llvm "ret i64 %c\n"
..

# Zero-extends u32 to u64.
# @complexity O(1).
# @example
#   wide := cast.u32to64(value)
pub u32to64(x u32) u64:
    llvm "%c = zext i32 %x to i64\n"
    llvm "ret i64 %c\n"
..

# Zero-extends u16 to u64.
# @complexity O(1).
# @example
#   wide := cast.u16to64(value)
pub u16to64(x u16) u64:
    llvm "%c = zext i16 %x to i64\n"
    llvm "ret i64 %c\n"
..

# Zero-extends u8 to u64.
# @complexity O(1).
# @example
#   wide := cast.u8to64(value)
pub u8to64(x u8) u64:
    llvm "%c = zext i8 %x to i64\n"
    llvm "ret i64 %c\n"
..

# Truncates i64 to i32.
# @warning higher bits are discarded on overflow.
# @complexity O(1).
# @example
#   narrow := cast.i64to32(value)
pub i64to32(x i64) i32:
    llvm "%c = trunc i64 %x to i32\n"
    llvm "ret i32 %c\n"
..

# Truncates i64 to i16.
# @warning higher bits are discarded on overflow.
# @complexity O(1).
# @example
#   narrow := cast.i64to16(value)
pub i64to16(x i64) i16:
    llvm "%c = trunc i64 %x to i16\n"
    llvm "ret i16 %c\n"
..

# Truncates i64 to i8.
# @warning higher bits are discarded on overflow.
# @complexity O(1).
# @example
#   narrow := cast.i64to8(value)
pub i64to8(x i64) i8:
    llvm "%c = trunc i64 %x to i8\n"
    llvm "ret i8 %c\n"
..

# Truncates u64 to u32.
# @warning higher bits are discarded on overflow.
# @complexity O(1).
# @example
#   narrow := cast.u64to32(value)
pub u64to32(x u64) u32:
    llvm "%c = trunc i64 %x to i32\n"
    llvm "ret i32 %c\n"
..

# Truncates u64 to u16.
# @warning higher bits are discarded on overflow.
# @complexity O(1).
# @example
#   narrow := cast.u64to16(value)
pub u64to16(x u64) u16:
    llvm "%c = trunc i64 %x to i16\n"
    llvm "ret i16 %c\n"
..

# Truncates u64 to u8.
# @warning higher bits are discarded on overflow.
# @complexity O(1).
# @example
#   narrow := cast.u64to8(value)
pub u64to8(x u64) u8:
    llvm "%c = trunc i64 %x to i8\n"
    llvm "ret i8 %c\n"
..
