# `std/net/tcp`

Owned convenience wrappers for TCP.

```magma
listener := try tcp.listen(address.anyIpv4(8080), 128)
defer listener.close()
stream := try listener.accept()
defer stream.close()
```

- `listen(endpoint, backlog) !$Listener` opens, enables address reuse, binds,
  and listens. `Listener.accept()` returns an owned `Stream`.
- `connect(endpoint) !$Stream` opens and connects a client stream.
- Both types expose `localEndpoint` and `setNonBlocking`; streams additionally
  expose `peerEndpoint`, `shutdown`, and borrowed `reader`, `writer`, and
  `duplex` views.
- `Listener.close()` and `Stream.close()` release their sockets.

The public `socket` field permits lower-level operations when required.
