mod net_tcp
# TCP listener and stream conveniences.

use "std:net/address" address
use "std:net/socket" socket
use "std:reader" reader
use "std:writer" writer
use "std:duplex" duplex

pub Listener(
    socket socket.Socket
)

pub Stream(
    socket socket.Socket
)

pub listen(endpoint address.Endpoint, backlog u32) !$Listener:
    value := try socket.open(endpoint.address.family, socket.TYPE_STREAM)
    onerror value.close()
    try value.setReuseAddress(true)
    try value.bind(endpoint)
    try value.listen(backlog)
    ret Listener(socket=value)
..

pub connect(endpoint address.Endpoint) !$Stream:
    value := try socket.open(endpoint.address.family, socket.TYPE_STREAM)
    onerror value.close()
    try value.connect(endpoint)
    ret Stream(socket=value)
..

Listener.accept() !$Stream:
    accepted := try this.socket.accept()
    ret Stream(socket=accepted)
..

Listener.localEndpoint() !address.Endpoint:
    ret try this.socket.localEndpoint()
..

Listener.setNonBlocking(enabled bool) !void:
    try this.socket.setNonBlocking(enabled)
..

destr Listener.close() !void:
    try this.socket.close()
..

Stream.reader() !reader.Reader:
    ret try this.socket.reader()
..

Stream.writer() !writer.Writer:
    ret try this.socket.writer()
..

Stream.duplex() !duplex.Duplex:
    ret try this.socket.duplex()
..

Stream.localEndpoint() !address.Endpoint:
    ret try this.socket.localEndpoint()
..

Stream.peerEndpoint() !address.Endpoint:
    ret try this.socket.peerEndpoint()
..

Stream.setNonBlocking(enabled bool) !void:
    try this.socket.setNonBlocking(enabled)
..

Stream.shutdown(direction u8) !void:
    try this.socket.shutdown(direction)
..

destr Stream.close() !void:
    try this.socket.close()
..
