# `std/net/tls`

Nonblocking client-side TLS over a connected `std/net/socket.Socket`.
Windows and Linux have native implementations; other currently declared
targets report unsupported operation.

- `newContext() !$Context` creates a reusable client TLS context.
- `Context.open(a, socket, host) !$Session` creates a session over a borrowed,
  connected transport. The socket and allocator must remain valid until the
  session is closed. `host` is used for server-name indication and certificate
  hostname verification.
- `Session.handshake()` advances negotiation and returns whether it completed.
- `Session.send` and `recv` return bytes processed.
- `Session.want` is `WANT_NONE`, `WANT_READ`, or `WANT_WRITE`, indicating the
  readiness needed before retrying a nonblocking operation.
- Close each `Session` before its transport, and close the `Context` after all
  sessions.

Handshake and I/O calls can make partial progress. Drive them again when the
socket has the readiness indicated by `want`.
