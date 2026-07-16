# `std/cast`

## Example

```magma
small u8 = cast.u64to8(258) # 2
address u64 = cast.ptou(pointer)
pointer = cast.utop(address)
```

Explicit primitive and pointer conversions. These operations do not allocate or validate that a value is representable in the destination type.

## Pointer casts

- `pub reinterpret[T](x ptr) T*` treats a raw pointer as `T*` without changing its address.
- `pub ptou(x ptr) u64` converts a pointer to its integer address.
- `pub utop(x u64) ptr` converts an integer address to a raw pointer.

## Numeric casts

- `pub itou(x i64) u64` and `pub utoi(x u64) i64` preserve the bit pattern between signed and unsigned 64-bit integers.
- `pub u64to128(x u64) u128` zero-extends; `pub u128to64(x u128) u64` truncates.
- `pub i64to128(x i64) i128` sign-extends; `pub i128to64(x i128) i64` truncates.
- `pub itof(x i64) f64`, `pub utof(x u64) f64`, `pub ftoi(x f64) i64`, and `pub ftou(x f64) u64` convert between integers and `f64`.
- `pub i32to64(x i32) i64`, `pub i16to64(x i16) i64`, and `pub i8to64(x i8) i64` sign-extend.
- `pub u32to64(x u32) u64`, `pub u16to64(x u16) u64`, and `pub u8to64(x u8) u64` zero-extend.
- `pub i64to32(x i64) i32`, `pub i64to16(x i64) i16`, and `pub i64to8(x i64) i8` truncate high bits.
- `pub u64to32(x u64) u32`, `pub u64to16(x u64) u16`, and `pub u64to8(x u64) u8` truncate high bits.
