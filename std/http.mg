mod http
# HTTP client requests and streaming response bodies.
# @platform windows

use "std:allocator" alc
use "std:reader"    reader
use "std:cast"      cast
use "std:builder"   builder
use "std:strings"   strings
use "std:slices"    slices
use "std:errors"    errors
use "std:memory"    memory

@platform("windows")
use "std:win/http_impl" impl_http

# HTTP is intentionally Windows-only until the standard library has portable
# TCP and TLS stream abstractions on which the non-Windows client can be built.

drop[T](value $T) void:
    abandoned := array T[1]
    abandoned[0] = value
    ret
..

# One HTTP request header. Names and values are borrowed for the duration of
# Client.send.
pub Header(
    name str
    value str
)

# Request metadata. The URL must be absolute and use http or https.
pub Request(
    method str
    url str
    headers Header[]
)

# A request body is either absent or a known-length Reader. Keeping the length
# explicit lets WinHTTP stream without first buffering the body.
pub Body(
    source reader.Reader
    length u64
)

# Creates an absent request body for methods such as GET.
# @complexity O(1)
# @example
#   response := try client.send(request, http.noBody())
pub noBody() Body:
    ret Body(source=memory.zeroValue[reader.Reader](), length=0)
..

# Creates a streaming request body with an exact byte length.
# @warning source must remain valid until send() returns and must produce length bytes.
# @complexity O(1)
# @example
#   requestBody := http.body(file.reader(), contentLength)
pub body(source reader.Reader, length u64) Body:
    ret Body(source=source, length=length)
..

# Reports whether this body contains a reader.
# @complexity O(1)
# @example
#   hasBody := requestBody.isPresent()
Body.isPresent() bool:
    ret this.source.fn_read != none
..

# Client session configuration. Timeouts are expressed in milliseconds and
# apply separately to connection establishment, sending, and receiving.
# @example
#   options := http.defaultOptions()
pub Options(
    userAgent str
    connectTimeoutMs u32
    sendTimeoutMs u32
    receiveTimeoutMs u32
    automaticDecompression bool
)

# Returns practical client defaults, including 30-second phase timeouts and
# automatic response decompression.
# @complexity O(1)
# @example
#   client := try http.new(a, http.defaultOptions())
pub defaultOptions() Options:
    ret Options(
        userAgent="Magma/0",
        connectTimeoutMs=30000,
        sendTimeoutMs=30000,
        receiveTimeoutMs=30000,
        automaticDecompression=true,
    )
..

# Reusable HTTP client session. Reuse one client across requests to benefit
# from the platform's connection pooling.
# @ownership A Client returned by new() must be closed exactly once.
pub Client(
    impl impl_http.Client
    allocator alc.Allocator
)

# Opens a reusable WinHTTP session.
# @platform windows
# @complexity O(1), excluding operating-system setup
# @ownership The returned client must be closed.
# @example
#   client := try http.new(a, http.defaultOptions())
pub new(a alc.Allocator, options Options) !$Client:
    impl := try impl_http.openClient(a, options.userAgent, options.connectTimeoutMs, options.sendTimeoutMs, options.receiveTimeoutMs, options.automaticDecompression)
    c Client
    c.impl = impl
    c.allocator = a
    ret c
..

# Closes the underlying HTTP session and invalidates the client.
# @complexity O(1)
# @example
#   client.close()
destr Client.close() void:
    impl_http.closeClient(addrof this.impl)
..

# Streaming HTTP response containing status, headers, and an unread body.
# @ownership A Response returned by send(), get(), or post() must be closed.
pub Response(
    impl impl_http.Response
)

# Returns the numeric HTTP response status, such as 200 or 404.
# @complexity O(1)
# @example
#   if response.statusCode() == 200:
Response.statusCode() u16:
    ret this.impl.statusCode
..

# Raw response headers, including the HTTP status line, encoded as UTF-8.
# The returned string is borrowed from the Response.
# @complexity O(1)
# @ownership The returned view becomes invalid when the response is closed.
# @example
#   headers := response.rawHeaders()
Response.rawHeaders() str:
    ret this.impl.rawHeaders
..

# Returns a pull reader backed directly by WinHttpReadData.
# The Response must remain alive and unmoved while the reader is in use.
# @complexity O(1)
# @ownership The reader borrows the response and must not outlive it.
# @example
#   bodyReader := try response.body()
Response.body() !reader.Reader:
    ret try impl_http.responseBody(addrof this.impl)
..

# Closes the response stream and invalidates its body reader and borrowed headers.
# @complexity O(1)
# @example
#   response.close()
destr Response.close() void:
    impl_http.closeResponse(addrof this.impl)
..

# Sends request headers and streams the request body before returning as soon
# as response headers are available. The response body is not buffered.
# @complexity O(H + B) before returning, for header and request-body byte counts
# @ownership The returned response must be closed; request inputs remain caller-owned.
# @example
#   response := try client.send(request, http.noBody())
Client.send(request Request, requestBody Body) !$Response:
    headerBuilder := try builder.new(this.allocator)
    defer headerBuilder.free()

    headers Header[] = request.headers
    i u64 = 0
    while i < slices.count(headers):
        try headerBuilder.appendBorrowed(headers[i].name)
        try headerBuilder.appendBorrowed(": ")
        try headerBuilder.appendBorrowed(headers[i].value)
        try headerBuilder.appendBorrowed("\r\n")
        i = i + 1
    ..

    rawHeaders str = try headerBuilder.build()

    response Response
    bodyPresent bool = requestBody.isPresent()
    responseImpl impl_http.Response, sendErr error = impl_http.send(addrof this.impl, request.method, request.url, rawHeaders, requestBody.source, requestBody.length, bodyPresent)
    strings.free(this.allocator, rawHeaders)

    if sendErr.nok():
        drop[Response](response)
        throw sendErr
    ..
    response.impl = responseImpl
    ret response
..

# Convenience GET. It still returns a streaming Response.
# @complexity O(N) for request setup and network transmission
# @ownership The returned response must be closed.
# @example
#   response := try client.get("https://example.com/")
Client.get(url str) !$Response:
    headers Header[] = slices.fromPtr(none, 0)
    request := Request(method="GET", url=url, headers=headers)
    ret try this.send(request, noBody())
..

# Sends a POST request with a known-length streaming body.
# @complexity O(B) for request-body bytes before response headers arrive
# @ownership The returned response must be closed.
# @example
#   response := try client.post(url, http.body(source, length))
Client.post(url str, requestBody Body) !$Response:
    headers Header[] = slices.fromPtr(none, 0)
    request := Request(method="POST", url=url, headers=headers)
    ret try this.send(request, requestBody)
..
