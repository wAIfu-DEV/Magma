# `std/net/socket`

Low-level owned stream and datagram sockets.

`open(family, kind) !$Socket` accepts an address family from
`std/net/address` and either `TYPE_STREAM` or `TYPE_DATAGRAM`. `Socket.close()`
consumes the native resource and is safe to call on an already closed value.

## Operations

- `bind`, `listen`, `accept`, and `connect` manage endpoints and connections.
- `setNonBlocking` and `setReuseAddress` configure a socket.
- `localEndpoint` and `peerEndpoint` inspect bound endpoints.
- `recv`/`send` handle connected traffic; `recvFrom`/`sendTo` handle datagrams.
  `Received` contains the byte count and source endpoint.
- `shutdown` accepts `SHUTDOWN_READ`, `SHUTDOWN_WRITE`, or `SHUTDOWN_BOTH`.
- `reader`, `writer`, and `duplex` return borrowed protocol views. The socket
  must outlive every use of a view.
- `nativeHandle` exposes a borrowed opaque handle for integration code.

For receive methods, `count` must not exceed the supplied buffer length.
Sending can complete partially; callers that require the whole payload must
continue until all bytes have been sent. Nonblocking operations may report
would-block.
