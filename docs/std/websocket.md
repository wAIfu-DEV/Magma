# `std/websocket`

Blocking RFC 6455 client for plain `ws://` connections. The client performs the
HTTP opening handshake, masks every client frame with operating-system random
bytes, validates server framing, replies to ping frames, and supports text,
binary, pong, and close frames. `wss://` is recognized but currently reports an
unsupported-operation failure while the owned TLS transport integration is
being completed.

```magma
headers websocket.Header[] = slices.fromPtr(none, 0)
client := try websocket.connect("ws://127.0.0.1:8080/socket", headers, websocket.defaultOptions())
defer client.close()

try client.sendText("hello")
message := try client.receive()
defer message.close()
```

`Options` bounds response headers and message payloads. `Message.kind` is
`MESSAGE_TEXT` or `MESSAGE_BINARY`; `Message.data` owns the received payload.
Close every successfully created `Message` and `Client`.

`Client` implements `std/reader.Reader` and `std/writer.Writer`. `write` sends
one binary WebSocket message. `readRaw` exposes received text and binary
payloads as a byte stream, retaining the unread portion of a message for the
next read. The borrowed `client.reader()` and `client.writer()` helpers return
the corresponding generic interface views.

The initial implementation accepts complete, unfragmented messages. WebSocket
extensions and fragmented messages are rejected rather than interpreted
loosely.
