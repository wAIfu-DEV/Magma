mod cast

pub itou(x i64) u64:
    llvm "ret i64 %x\n"
..

pub utoi(x u64) i64:
    llvm "ret i64 %x\n"
..

pub itof(x i64) f64:
    llvm "%c = sitofp i64 %x to double\n"
    llvm "ret double %c\n"
..

pub utof(x i64) f64:
    llvm "%c = uitofp i64 %x to double\n"
    llvm "ret double %c\n"
..

pub ftoi(x f64) i64:
    llvm "%c = fptosi double %x to i64\n"
    llvm "ret i64 %c\n"
..

pub ftou(x f64) u64:
    llvm "%c = fptoui double %x to i64\n"
    llvm "ret i64 %c\n"
..

pub i32to64(x i32) i64:
    llvm "%c = sext i32 %x to i64\n"
    llvm "ret i64 %c\n"
..

pub i16to64(x i16) i64:
    llvm "%c = sext i16 %x to i64\n"
    llvm "ret i64 %c\n"
..

pub i8to64(x i8) i64:
    llvm "%c = sext i8 %x to i64\n"
    llvm "ret i64 %c\n"
..

pub u32to64(x u32) u64:
    llvm "%c = zext i32 %x to i64\n"
    llvm "ret i64 %c\n"
..

pub u16to64(x u16) u64:
    llvm "%c = zext i16 %x to i64\n"
    llvm "ret i64 %c\n"
..

pub u8to64(x u8) u64:
    llvm "%c = zext i8 %x to i64\n"
    llvm "ret i64 %c\n"
..

pub i64to32(x i64) i32:
    llvm "%c = trunc i64 %x to i32\n"
    llvm "ret i32 %c\n"
..

pub i64to16(x i64) i16:
    llvm "%c = trunc i64 %x to i16\n"
    llvm "ret i16 %c\n"
..

pub i64to8(x i64) i8:
    llvm "%c = trunc i64 %x to i8\n"
    llvm "ret i8 %c\n"
..

pub u64to32(x u64) u32:
    llvm "%c = trunc i64 %x to i32\n"
    llvm "ret i32 %c\n"
..

pub u64to16(x u64) u16:
    llvm "%c = trunc i64 %x to i16\n"
    llvm "ret i16 %c\n"
..

pub u64to8(x u64) u8:
    llvm "%c = trunc i64 %x to i8\n"
    llvm "ret i8 %c\n"
..
