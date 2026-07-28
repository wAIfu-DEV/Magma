# `std/c`

Target-dependent aliases for C interoperability. The module exports signed and
unsigned `char`, `short`, `int`, `long`, and `long_long` variants. Their exact
names are `char`, `signed_char`, `unsigned_char`, `short`, `unsigned_short`,
`int`, `unsigned_int`, `long`, `unsigned_long`, `long_long`, and
`unsigned_long_long`. The module also exports `size_t`,
`ptrdiff_t`, `intptr_t`, `uintptr_t`, and `wchar_t`.

Use these aliases in external declarations and C-compatible layouts instead of
assuming a Magma integer width. C `long` varies by target ABI. The aliases are
compiler-known types and introduce no runtime conversion.
