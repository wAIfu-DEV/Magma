mod cast

# Casts a pointer to u64.
# O(1).
pub ptou(x ptr) u64:
    llvm "%x0 = ptrtoint ptr %x to i64\n"
    llvm "ret i64 %x0\n"
..

# Casts a u64 to pointer.
# O(1).
pub utop(x u64) ptr:
    llvm "%x0 = inttoptr i64 %x to ptr\n"
    llvm "ret ptr %x0\n"
..

# Casts i64 to u64.
# O(1).
pub itou(x i64) u64:
    llvm "ret i64 %x\n"
..

# Casts u64 to i64.
# O(1).
pub utoi(x u64) i64:
    llvm "ret i64 %x\n"
..

# Converts signed i64 to f64.
# O(1).
pub itof(x i64) f64:
    llvm "%c = sitofp i64 %x to double\n"
    llvm "ret double %c\n"
..

# Converts unsigned u64 to f64.
# O(1).
pub utof(x u64) f64:
    llvm "%c = uitofp i64 %x to double\n"
    llvm "ret double %c\n"
..

# Converts f64 to signed i64.
# O(1).
pub ftoi(x f64) i64:
    llvm "%c = fptosi double %x to i64\n"
    llvm "ret i64 %c\n"
..

# Converts f64 to unsigned u64.
# O(1).
pub ftou(x f64) u64:
    llvm "%c = fptoui double %x to i64\n"
    llvm "ret i64 %c\n"
..

# Sign-extends i32 to i64.
# O(1).
pub i32to64(x i32) i64:
    llvm "%c = sext i32 %x to i64\n"
    llvm "ret i64 %c\n"
..

# Sign-extends i16 to i64.
# O(1).
pub i16to64(x i16) i64:
    llvm "%c = sext i16 %x to i64\n"
    llvm "ret i64 %c\n"
..

# Sign-extends i8 to i64.
# O(1).
pub i8to64(x i8) i64:
    llvm "%c = sext i8 %x to i64\n"
    llvm "ret i64 %c\n"
..

# Zero-extends u32 to u64.
# O(1).
pub u32to64(x u32) u64:
    llvm "%c = zext i32 %x to i64\n"
    llvm "ret i64 %c\n"
..

# Zero-extends u16 to u64.
# O(1).
pub u16to64(x u16) u64:
    llvm "%c = zext i16 %x to i64\n"
    llvm "ret i64 %c\n"
..

# Zero-extends u8 to u64.
# O(1).
pub u8to64(x u8) u64:
    llvm "%c = zext i8 %x to i64\n"
    llvm "ret i64 %c\n"
..

# Truncates i64 to i32.
# Warning: higher bits are discarded on overflow.
# O(1).
pub i64to32(x i64) i32:
    llvm "%c = trunc i64 %x to i32\n"
    llvm "ret i32 %c\n"
..

# Truncates i64 to i16.
# Warning: higher bits are discarded on overflow.
# O(1).
pub i64to16(x i64) i16:
    llvm "%c = trunc i64 %x to i16\n"
    llvm "ret i16 %c\n"
..

# Truncates i64 to i8.
# Warning: higher bits are discarded on overflow.
# O(1).
pub i64to8(x i64) i8:
    llvm "%c = trunc i64 %x to i8\n"
    llvm "ret i8 %c\n"
..

# Truncates u64 to u32.
# Warning: higher bits are discarded on overflow.
# O(1).
pub u64to32(x u64) u32:
    llvm "%c = trunc i64 %x to i32\n"
    llvm "ret i32 %c\n"
..

# Truncates u64 to u16.
# Warning: higher bits are discarded on overflow.
# O(1).
pub u64to16(x u64) u16:
    llvm "%c = trunc i64 %x to i16\n"
    llvm "ret i16 %c\n"
..

# Truncates u64 to u8.
# Warning: higher bits are discarded on overflow.
# O(1).
pub u64to8(x u64) u8:
    llvm "%c = trunc i64 %x to i8\n"
    llvm "ret i8 %c\n"
..
