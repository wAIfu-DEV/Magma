# `std/http`

Portable buffered HTTP/1.1 client with synchronous and asynchronous requests,
response-size limits, and client-owned keep-alive connection pooling. Plain
`http` uses the portable socket stack; `https` has native TLS implementations
on Windows and Linux and reports unsupported operation on the other targets.

```magma
headers http.Header[] = slices.fromPtr(none, 0)
request := http.noBody("GET", "https://example.com/", headers)
client := try http.new(heap.allocator(), http.defaultOptions())
defer client.close()
response := try client.send(request)
defer response.close()
status := response.statusCode
body := response.body
```

## Requests and options

- `Header(name str, value str)` is a borrowed request-header pair.
- `Request(method, url, headers, body, bodyLength)` describes a request.
- `noBody(method, url, headers)` creates a request without a body.
- `Options` controls DNS, I/O timeout, response and read-buffer limits, and
  connection-pool capacity. `defaultOptions()` supplies practical defaults.

Request bodies have a known length and are buffered into the serialized
request before transmission. Responses are buffered up to `maxResponseBytes`.

## Client and responses

- `new(allocator, options)` creates a reusable client.
- `Client.start(request)` creates a manually polled `Exchange`.
- `Exchange.poll(timeoutMs)` advances one readiness cycle.
- `Exchange.finish()` returns the completed buffered `Response`.
- `Client.send(request)` performs a synchronous exchange.
- `Client.sendAsync(request)` runs an exchange through the allocator and
  executor in implicit `ctx` and returns `Future[Response]`.
- `Response.statusCode`, `Response.rawHeaders`, and `Response.body` contain the
  buffered response data.

Close every `Response`, `Exchange`, and `Client` that is successfully created.
HTTPS verifies the peer chain and hostname and uses the platform TLS backend.
