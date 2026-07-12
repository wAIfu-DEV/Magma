# `std/reader`

A type-erased byte-input interface.

## Type

`Reader(impl ptr, fn_read (ptr, u8[], u64) !u64)` pairs adapter state with a callback. The state must remain valid for the reader's lifetime.

## API

- `pub new(impl ptr, readFunc (ptr, u8[], u64) !u64) Reader` constructs an interface.
- `Reader.read(a alc.Allocator, nBytes u64) !$str` allocates space, reads up to `nBytes`, and returns an owned string containing exactly the bytes obtained.
- `Reader.readToBuff(buff u8[], nBytes u64) !u64` reads into caller storage and returns the byte count. `nBytes` must not exceed the slice length.

Read behavior and errors otherwise come from the adapter callback.
