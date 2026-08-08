mod net_listener
# Callback-driven TCP listener with synchronous and worker-pool execution modes.

use "std:allocator" allocator
use "std:async" async
use "std:errors" errors
use "std:net/address" address
use "std:net/event_loop" event_loop
use "std:net/poll" poll
use "std:net/tcp" tcp

pub alias AcceptCallback = (ptr, $tcp.Stream) !void

State(
    allocator allocator.Allocator
    server tcp.Listener
    callback AcceptCallback
    context ptr
)

pub Listener(
    state State*
    loop event_loop.EventLoop
)

pub RunningListener(
    state State*
    loop event_loop.RunningLoop
    active bool
)

claim[T](value $T) $T:
    ret value
..

onReady(raw ptr, token u64, flags u32) !void:
    state State* = raw
    if (flags & poll.READ) == 0:
        ret
    ..
    stream tcp.Stream, acceptError error = state.server.accept()
    if acceptError.nok():
        if errors.hasCode(acceptError, errors.ERR_WOULD_BLOCK):
            ret
        ..
        throw acceptError
    ..
    try state.callback(state.context, stream)
..

pub new(a allocator.Allocator, endpoint address.Endpoint, backlog u32, capacity u64, commandCapacity u64, callback AcceptCallback, context ptr) !$Listener:
    if callback == none:
        throw errors.invalidArgument("accept callback cannot be none")
    ..
    state State* = try a.allocT[State](1)
    onerror a.free(state)
    server := try tcp.listen(endpoint, backlog)
    onerror server.close()
    try server.setNonBlocking(true)
    loop := try event_loop.new(a, capacity, commandCapacity)
    onerror loop.close()
    state.allocator = a
    state.server = claim[tcp.Listener](server)
    state.callback = callback
    state.context = context
    try loop.watch(addrof state.server.socket, 0, poll.READ, onReady, state)
    ret Listener(state=state, loop=loop)
..

Listener.localEndpoint() !address.Endpoint:
    if this.state == none:
        throw errors.invalidArgument("listener is not active")
    ..
    ret try this.state.server.localEndpoint()
..

Listener.run() !void:
    try this.loop.run()
..

Listener.runOnce(timeoutMs i64) !u64:
    ret try this.loop.runOnce(timeoutMs)
..

Listener.stop() !void:
    try this.loop.stop()
..

destr Listener.runAsync(asc async.Async) !$RunningListener:
    if this.state == none:
        throw errors.invalidArgument("listener is not active")
    ..
    running := try this.loop.runAsync(asc)
    state := this.state
    this.state = none
    ret RunningListener(state=state, loop=running, active=true)
..

RunningListener.stop() !void:
    if this.active:
        try this.loop.stop()
    ..
..

closeState(state State*) !void:
    a := state.allocator
    try state.server.close()
    a.free(state)
..

closeStateResult(state State*) !bool:
    try closeState(state)
    ret true
..

destr RunningListener.await() !void:
    if this.active == false:
        throw errors.invalidArgument("running listener is not active")
    ..
    completed bool, runError error = awaitLoop(addrof this.loop)
    closed bool, closeError error = closeStateResult(this.state)
    this.state = none
    this.active = false
    if runError.nok():
        throw runError
    ..
    if closeError.nok():
        throw closeError
    ..
..

awaitLoop(loop event_loop.RunningLoop*) !bool:
    try loop.await()
    ret true
..

destr Listener.close() !void:
    if this.state != none:
        try this.loop.close()
        try closeState(this.state)
        this.state = none
    ..
..
