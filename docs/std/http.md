# `std/http`

A synchronous, streaming HTTP client backed by WinHTTP. The module currently
supports Windows only.

## Request bodies

- `pub noBody() Body` creates an absent request body.
- `pub body(source reader.Reader, length u64) Body` creates a known-length
  streaming body. `Client.send` pulls exactly `length` bytes and fails if the
  reader reaches EOF early.

Unknown-length/chunked uploads are not implemented yet.

## Client

- `pub defaultOptions() Options` enables automatic gzip/deflate decompression
  and sets 30-second connect, send, and receive timeouts.
- `pub new(a alc.Allocator, options Options) !$Client` opens a reusable WinHTTP
  session.
- `Client.send(request Request, body Body) !$Response` streams the upload and
  returns after response headers arrive. It does not buffer the response body.
- `Client.get(url str) !$Response` is a streaming GET convenience method.
- `destr Client.close()` closes the WinHTTP session.

## Response

- `Response.statusCode() u16` returns the numeric HTTP status.
- `Response.rawHeaders() str` borrows the UTF-8 response header block.
- `Response.body() !reader.Reader` returns a reader backed by WinHTTP. A zero
  byte read indicates EOF.
- `destr Response.close()` closes native handles and frees owned headers.

The response must remain alive and unmoved while its body reader is used. Close
every response, including responses whose status is an error. Operations are
synchronous and may block according to the configured timeouts.
