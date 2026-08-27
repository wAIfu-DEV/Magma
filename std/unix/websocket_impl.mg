mod websocket_impl_unix

use "std:c" c
use "std:slices" slices

ext ext_unix_arc4random_buf arc4random_buf(buffer ptr, count c.size_t) void

pub randomBytes(output u8[]) !void:
    ext_unix_arc4random_buf(slices.toPtr(output), slices.count(output))
..
