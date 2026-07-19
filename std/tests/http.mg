mod main

use "../http.mg" http
use "../cast.mg" cast
use "../slices.mg" slices
use "../allocator.mg" allocator
use "../heap.mg" heap
use "../reader.mg" reader
use "../errors.mg" errors
use "../memory.mg" memory
use "../strings.mg" strings

emptyRead(impl ptr, bytes u8[], count u64) !u64: ret 0 ..

pub main() !void:
    options := http.defaultOptions()
    if options.connectTimeoutMs == 0:
        throw errors.failure("default HTTP timeout is zero")
    ..
    body := http.noBody()
    if body.isPresent():
        throw errors.failure("empty HTTP body is present")
    ..
    source := reader.new(none, emptyRead)
    present := http.body(source, 0)
    if present.isPresent() == false:
        throw errors.failure("HTTP reader body is absent")
    ..
    headers http.Header[] = slices.fromPtr(none, 0)
    request := http.Request(method="GET", url="https://example.com/", headers=headers)
    a allocator.Allocator = heap.allocator()
    client := try http.new(a, options)
    invalidRequest := http.Request(method="GET", url="://", headers=headers)
    failedResponse http.Response, sendErr error = client.send(invalidRequest, http.noBody())
    if sendErr.ok():
        failedResponse.close()
        client.close()
        throw errors.failure("HTTP send accepted an invalid URL")
    ..
    failedGet http.Response, getErr error = client.get("://")
    if getErr.ok():
        failedGet.close()
        client.close()
        throw errors.failure("HTTP get accepted an invalid URL")
    ..
    failedPost http.Response, postErr error = client.post("://", http.noBody())
    if postErr.ok():
        failedPost.close()
        client.close()
        throw errors.failure("HTTP post accepted an invalid URL")
    ..
    client.close()
..
