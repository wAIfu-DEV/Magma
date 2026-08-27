# `std/sha1`

SHA-1 digest support for compatibility protocols such as the WebSocket opening
handshake. SHA-1 is deprecated for signatures and new cryptographic designs.

`sum(input, output)` writes the 20-byte digest into caller-owned storage.
