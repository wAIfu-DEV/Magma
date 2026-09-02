mod main

use "std:websocket" websocket
use "std:errors" errors
use "std:reader" reader
use "std:slices" slices
use "std:writer" writer

# Compile-time coverage for Client's generic byte-stream adapters.
checkInterfaces(client websocket.Client*) !void:
    input reader.Reader = client.reader()
    output writer.Writer = client.writer()
    try client.writeAll("")
..

pub main() !void:
    options := websocket.defaultOptions()
    if options.maxHeaderBytes == 0 || options.maxMessageBytes == 0:
        throw errors.failure("default WebSocket limits are invalid")
    ..
    headers websocket.Header[] = slices.fromPtr(none, 0)
    client websocket.Client, failure error = websocket.connect("http://invalid", headers, options)
    if failure.ok():
        client.close()
        throw errors.failure("WebSocket accepted a non-WebSocket URL")
    ..
..
