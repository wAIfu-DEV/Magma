mod main
# Minimal callback-based echo server. The listener runs on a worker thread and
# the main thread can perform unrelated work before stopping and awaiting it.

use "std:context" context
use "std:heap" heap
use "std:thread_pool" thread_pool
use "std:time" time
use "std:net/address" address
use "std:net/listener" listener
use "std:net/tcp" tcp

onClient(c ptr, stream $tcp.Stream) !void:
    defer stream.close()
    input := try stream.reader()
    output := try stream.writer()
    a := heap.allocator()
    bytes := try input.read(4096)
    defer bytes.free(a)
    try output.writeAll(bytes)
..

pub main() !void:
    a := heap.allocator()
    pool := try thread_pool.new(a, 1, 4, 256, 256)
    defer pool.close()

    ctx = context.new(a, a, pool.executor())

    server := try listener.new(ctx.procAlloc, address.anyIpv4(7000), 128, 1024, 256, onClient, none)
    running := try server.runAsync()

    # A real application would do useful work or wait for a shutdown signal.
    time.sleep(10000)
    try running.stop()
    try running.await()
..
