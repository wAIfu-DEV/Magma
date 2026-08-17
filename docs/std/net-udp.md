# `std/net/udp`

Owned UDP datagram sockets.

- `bind(endpoint) !$Datagram` opens and binds a datagram socket.
- `open(family) !$Datagram` opens an unbound IPv4 or IPv6 socket.
- `connect(endpoint)` sets the default peer at the socket layer.
- `recvFrom(buffer, count)` returns `socket.Received`; `sendTo(bytes, endpoint)`
  returns the number of bytes sent.
- `localEndpoint` inspects the bound address, `setNonBlocking` changes blocking
  mode, and `close` releases the socket.

`count` must fit the receive buffer. The public `socket` field is available for
connected send/receive or other low-level operations.
