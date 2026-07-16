# `std/hash`

## Example

```magma
textHash := hash.string("magma")
byteHash := hash.bytes(bytes)
```

Non-cryptographic FNV-1a hashing.

- `pub bytes(in u8[]) u64` hashes a byte slice to a 64-bit value.
- `pub string(in str) u64` hashes a string's bytes to a 64-bit value.

These hashes are suitable for hash tables, not cryptographic use.
