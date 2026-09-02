mod net_byte_order
# Pure helpers for reading and writing integer fields in network byte order.

use "std:cast" cast

# Writes a u16 to native storage in big-endian (network) byte order.
pub store16(target u16*, value u16) void:
    # SAFETY: target points to a writable u16, which is exactly two bytes.
    unsafe:
        bytes u8* = target
        bytes[0] = cast.u64to8(value >> 8)
        bytes[1] = cast.u64to8(value & 0xFF)
    ..
..

# Reads a big-endian (network-order) u16 from native storage.
pub load16(source u16*) u16:
    # SAFETY: source points to a readable u16, which is exactly two bytes.
    unsafe:
        bytes u8* = source
        ret (cast.u64to16(bytes[0]) << 8) | cast.u64to16(bytes[1])
    ..
..

# Writes a u32 to native storage in big-endian (network) byte order.
pub store32(target u32*, value u32) void:
    # SAFETY: target points to a writable u32, which is exactly four bytes.
    unsafe:
        bytes u8* = target
        bytes[0] = cast.u64to8(value >> 24)
        bytes[1] = cast.u64to8((value >> 16) & 0xFF)
        bytes[2] = cast.u64to8((value >> 8) & 0xFF)
        bytes[3] = cast.u64to8(value & 0xFF)
    ..
..

# Reads a big-endian (network-order) u32 from native storage.
pub load32(source u32*) u32:
    # SAFETY: source points to a readable u32, which is exactly four bytes.
    unsafe:
        bytes u8* = source
        ret (cast.u64to32(bytes[0]) << 24) | (cast.u64to32(bytes[1]) << 16) | (cast.u64to32(bytes[2]) << 8) | cast.u64to32(bytes[3])
    ..
..
