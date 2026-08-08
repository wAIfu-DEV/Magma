mod main

use "std:allocator" allocator
use "std:async" async
use "std:atomic" atomic
use "std:builder" builder
use "std:cast" cast
use "std:errors" errors
use "std:heap" heap
use "std:slices" slices
use "std:strconv" strconv
use "std:strings" strings
use "std:thread" thread
use "std:thread_pool" thread_pool
use "std:time" time
use "std:net/address" address
use "std:net/dns" dns
use "std:net/event_loop" event_loop
use "std:net/listener" net_listener
use "std:net/poll" poll
use "std:net/socket" socket
use "std:net/tcp" tcp
use "std:net/udp" udp
use "std:http" http_client

CallbackContext(socket socket.Socket*, calls atomic.U64)
AcceptContext(calls atomic.U64)
HttpContext(calls atomic.U64, accepts atomic.U64)

onReadable(raw ptr, token u64, flags u32) !void:
    context CallbackContext* = raw
    bytes := array u8[8]
    count := try context.socket.recv(bytes, 8)
    if token != 91 || (flags & poll.READ) == 0 || count != 1:
        throw errors.failure("event-loop callback received invalid readiness")
    ..
    context.calls.fetchAdd(1)
..

onAccept(raw ptr, stream $tcp.Stream) !void:
    context AcceptContext* = raw
    try stream.close()
    context.calls.fetchAdd(1)
..

onHttpAccept(raw ptr, stream $tcp.Stream) !void:
    context HttpContext* = raw
    context.accepts.fetchAdd(1)
    requestNumber u64 = 0
    loop requestNumber < 2:
        requestBytes := array u8[2048]
        received u64 = 0
        deadline := time.ticks() + time.msToTicks(2000)
        loop received == 0 && time.ticks() < deadline:
            count u64, receiveError error = stream.socket.recv(requestBytes, 2048)
            if receiveError.ok():
                received = count
            elif errors.hasCode(receiveError, errors.ERR_WOULD_BLOCK):
                thread.yield()
            else:
                throw receiveError
            ..
        ..
        if received == 0:
            throw errors.timedOut("HTTP test server did not receive request")
        ..
        response := "HTTP/1.1 200 OK\r\nContent-Length: 2\r\nConnection: keep-alive\r\n\r\nok"
        if requestNumber == 1:
            response = "HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\nConnection: keep-alive\r\n\r\n2\r\nok\r\n0\r\n\r\n"
        ..
        sent u64 = 0
        loop sent < response.countBytes():
            sent = sent + try stream.socket.send(strings.fromPtrNoCopy(cast.utop(cast.ptou(strings.toPtr(response)) + sent), response.countBytes() - sent))
        ..
        context.calls.fetchAdd(1)
        requestNumber = requestNumber + 1
    ..
    try stream.close()
..

testAddress() !void:
    parsed := try address.parseIpv4("127.0.0.1")
    if parsed.equal(address.ipv4(127, 0, 0, 1)) == false:
        throw errors.failure("IPv4 parser changed")
    ..
    parsed6 := try address.parseIpv6("2001:db8::1")
    expected6 := address.ipv6(0x20010DB8, 0, 0, 1)
    if parsed6.equal(expected6) == false:
        throw errors.failure("IPv6 parser changed")
    ..
..

testDns(a allocator.Allocator) !void:
    resolver := try dns.new(a, dns.defaultOptions())
    onerror resolver.close()
    output := array address.Endpoint[16]
    first := try resolver.resolveTo("localhost", "80", address.FAMILY_UNSPECIFIED, output)
    second := try resolver.resolveTo("localhost", "80", address.FAMILY_UNSPECIFIED, output)
    if first == 0 || second != first:
        throw errors.failure("DNS cache returned inconsistent results")
    ..
    try resolver.close()
..

testUdp() !void:
    receiver := try udp.bind(address.loopbackIpv4(0))
    onerror receiver.close()
    endpoint := try receiver.localEndpoint()
    sender := try udp.open(address.FAMILY_IPV4)
    onerror sender.close()
    sent := try sender.sendTo("udp", endpoint)
    bytes := array u8[8]
    received := try receiver.recvFrom(bytes, 8)
    if sent != 3 || received.count != 3 || received.source.address.isIpv4() == false:
        throw errors.failure("UDP loopback transfer failed")
    ..
    try sender.close()
    try receiver.close()
..

testTcpPollAndAsync(a allocator.Allocator) !void:
    listener := try tcp.listen(address.loopbackIpv4(0), 16)
    onerror listener.close()
    endpoint := try listener.localEndpoint()
    client := try tcp.connect(endpoint)
    onerror client.close()
    server := try listener.accept()
    onerror server.close()

    poller := try poll.new(a, 8)
    onerror poller.close()
    try poller.add(addrof server.socket, 77, poll.READ)
    try client.socket.send("p")
    events := array poll.Event[8]
    count := try poller.wait(events, 1000)
    if count == 0 || events[0].token != 77 || (events[0].flags & poll.READ) == 0:
        throw errors.failure("poller did not report socket readiness")
    ..
    byte := array u8[1]
    if try server.socket.recv(byte, 1) != 1:
        throw errors.failure("TCP loopback transfer failed")
    ..
    try poller.remove(addrof server.socket)
    try poller.close()

    loop := try event_loop.new(a, 8, 8)
    onerror loop.close()
    context := CallbackContext(socket=addrof server.socket, calls=atomic.newU64(0))
    pool := try thread_pool.new(a, 1, 1, 8, 64)
    onerror pool.close()
    asc := async.new(pool, a)
    running := try loop.runAsync(asc)
    onerror:
        running.stop()
        running.await()
    ..
    try running.watch(addrof server.socket, 91, poll.READ, onReadable, addrof context)
    try client.socket.send("e")
    deadline := time.ticks() + time.msToTicks(2000)
    loop context.calls.loadAcquire() == 0 && time.ticks() < deadline:
        thread.yield()
    ..
    if context.calls.loadAcquire() != 1:
        try running.stop()
        try running.await()
        try pool.close()
        throw errors.failure("asynchronous event loop did not dispatch")
    ..
    try running.stop()
    try running.await()
    try pool.close()
    try server.close()
    try client.close()
    try listener.close()
..

testAsyncListener(a allocator.Allocator) !void:
    context := AcceptContext(calls=atomic.newU64(0))
    listener := try net_listener.new(a, address.loopbackIpv4(0), 16, 8, 8, onAccept, addrof context)
    onerror listener.close()
    endpoint := try listener.localEndpoint()
    pool := try thread_pool.new(a, 1, 1, 8, 64)
    onerror pool.close()
    running := try listener.runAsync(async.new(pool, a))
    onerror:
        running.stop()
        running.await()
    ..
    client := try tcp.connect(endpoint)
    try client.close()
    deadline := time.ticks() + time.msToTicks(2000)
    loop context.calls.loadAcquire() == 0 && time.ticks() < deadline:
        thread.yield()
    ..
    try running.stop()
    try running.await()
    try pool.close()
    if context.calls.loadAcquire() != 1:
        throw errors.failure("asynchronous listener did not accept")
    ..
..

testHttpClient(a allocator.Allocator) !void:
    context := HttpContext(calls=atomic.newU64(0), accepts=atomic.newU64(0))
    listener := try net_listener.new(a, address.loopbackIpv4(0), 16, 8, 8, onHttpAccept, addrof context)
    onerror listener.close()
    endpoint := try listener.localEndpoint()
    pool := try thread_pool.new(a, 2, 2, 8, 64)
    onerror pool.close()
    asc := async.new(pool, a)
    running := try listener.runAsync(asc)
    onerror:
        running.stop()
        running.await()
    ..
    portText := try strconv.formatUint(a, endpoint.port)
    defer portText.free(a)
    urlBuilder := try builder.new(a)
    defer urlBuilder.free()
    try urlBuilder.appendBorrowed("http://127.0.0.1:")
    try urlBuilder.appendBorrowed(portText)
    try urlBuilder.appendBorrowed("/test")
    url := try urlBuilder.build()
    defer url.free(a)
    client := try http_client.new(a, http_client.defaultOptions())
    onerror client.close()
    headers http_client.Header[] = slices.fromPtr(none, 0)
    request := http_client.noBody("GET", url, headers)

    exchange := try client.start(request)
    onerror exchange.close()
    loop try exchange.poll(2000) == false:
    ..
    response := try exchange.finish()
    if response.statusCode != 200 || strings.compare(response.body, "ok") == false:
        throw errors.failure("polled HTTP response was invalid")
    ..
    response.close()

    pending := try client.sendAsync(asc, request)
    asyncResponse := try pending.await()
    if asyncResponse.statusCode != 200 || strings.compare(asyncResponse.body, "ok") == false:
        throw errors.failure("async HTTP response was invalid")
    ..
    asyncResponse.close()
    try client.close()
    try running.stop()
    try running.await()
    try pool.close()
    if context.calls.loadAcquire() != 2:
        throw errors.failure("HTTP test server did not receive both requests")
    ..
    if context.accepts.loadAcquire() != 1:
        throw errors.failure("HTTP client did not reuse its connection")
    ..
..

pub main() !void:
    a := heap.allocator()
    try testAddress()
    try testDns(a)
    try testUdp()
    try testTcpPollAndAsync(a)
    try testAsyncListener(a)
    try testHttpClient(a)
..
