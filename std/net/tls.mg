mod net_tls
# Nonblocking client-side TLS over a connected std/net socket.

use "std:allocator" allocator
use "std:net/socket" socket

@platform("linux")
use "std:linux/net/tls_impl" impl

@platform("windows")
use "std:win/net/tls_impl" impl

@platform("android", "ios", "darwin", "freebsd", "netbsd", "openbsd")
use "std:net/tls_unsupported" impl

pub const WANT_NONE u8 = 0
pub const WANT_READ u8 = 1
pub const WANT_WRITE u8 = 2

pub Context(
    native ptr
    active bool
)

pub Session(
    native ptr
    want u8
    handshaken bool
    active bool
)

pub newContext() !$Context:
    native := try impl.newContext()
    ret Context(native=native, active=true)
..

Context.open(a allocator.Allocator, transport socket.Socket*, host str) !$Session:
    if this.active == false:
        throw impl.closedContextError()
    ..
    native := try impl.open(this.native, a, transport, host)
    ret Session(native=native, want=WANT_WRITE, handshaken=false, active=true)
..

Session.handshake() !bool:
    result := try impl.handshake(this.native)
    this.want = result.want
    this.handshaken = result.complete
    ret result.complete
..

Session.send(bytes str) !u64:
    result := try impl.send(this.native, bytes)
    this.want = result.want
    ret result.count
..

Session.recv(buffer u8[], count u64) !u64:
    result := try impl.recv(this.native, buffer, count)
    this.want = result.want
    ret result.count
..

destr Session.close() !void:
    if this.active:
        try impl.close(this.native)
        this.native = none
        this.active = false
    ..
..

destr Context.close() !void:
    if this.active:
        try impl.closeContext(this.native)
        this.native = none
        this.active = false
    ..
..
