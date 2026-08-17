# Networking

Magma's networking modules provide portable IPv4/IPv6 addresses, sockets,
TCP, UDP, cached DNS, readiness polling, event loops, callback listeners, and
client-side TLS.

```magma
use "std:net/address" address
use "std:net/tcp" tcp

endpoint := address.loopbackIpv4(8080)
stream := try tcp.connect(endpoint)
defer stream.close()
try stream.socket.send("hello")
```

`Socket`, TCP/UDP wrappers, pollers, resolvers, loops, listeners, TLS contexts,
and TLS sessions own native or allocated resources. Close every successfully
created value. Methods that return `$T` transfer ownership to the caller.

The portable socket stack is selected on Windows, Linux, Android, Apple, and
the supported BSD targets. TLS is implemented on Windows and Linux; the TLS
module reports unsupported operation on the other targets. Nonblocking calls
can report `errors.ERR_WOULD_BLOCK`.

## Layers

- [`net/address`](net-address.md) defines allocation-free addresses/endpoints.
- [`net/socket`](net-socket.md) is the low-level owned socket API.
- [`net/tcp`](net-tcp.md) and [`net/udp`](net-udp.md) are convenience wrappers.
- [`net/dns`](net-dns.md) provides a bounded, caching resolver.
- [`net/poll`](net-poll.md) exposes portable readiness polling.
- [`net/event_loop`](net-event-loop.md) adds callbacks and async execution.
- [`net/listener`](net-listener.md) combines TCP accept with an event loop.
- [`net/tls`](net-tls.md) wraps a connected socket in client-side TLS.
