# `std/sha1`

SHA-1 digest support for compatibility protocols such as the WebSocket opening
handshake. SHA-1 is deprecated for signatures and new cryptographic designs.

`sum(input)` returns an allocated 20-byte digest using `ctx.alloc`.
