mod http

use "allocator.mg" alc
use "reader.mg"    reader
use "cast.mg"      cast
use "builder.mg"   builder
use "strings.mg"   strings
use "slices.mg"    slices
use "errors.mg"    errors
use "memory.mg"    memory

@platform("windows")
use "win/http_impl.mg" impl_http

# HTTP is intentionally Windows-only until the standard library has portable
# TCP and TLS stream abstractions on which the non-Windows client can be built.

drop[T](value $T) void:
    abandoned T[1]
    abandoned[0] = value
    ret
..

# One HTTP request header. Names and values are borrowed for the duration of
# Client.send.
Header(
    name str
    value str
)

# Request metadata. The URL must be absolute and use http or https.
Request(
    method str
    url str
    headers Header[]
)

# A request body is either absent or a known-length Reader. Keeping the length
# explicit lets WinHTTP stream without first buffering the body.
Body(
    source reader.Reader
    length u64
)

pub noBody() Body:
    ret Body(source=memory.zeroValue[reader.Reader](), length=0)
..

pub body(source reader.Reader, length u64) Body:
    ret Body(source=source, length=length)
..

Body.isPresent() bool:
    ret this.source.fn_read != none
..

Options(
    userAgent str
    connectTimeoutMs u32
    sendTimeoutMs u32
    receiveTimeoutMs u32
    automaticDecompression bool
)

pub defaultOptions() Options:
    ret Options(
        userAgent="Magma/0",
        connectTimeoutMs=30000,
        sendTimeoutMs=30000,
        receiveTimeoutMs=30000,
        automaticDecompression=true,
    )
..

Client(
    impl impl_http.Client
    allocator alc.Allocator
)

# Opens a reusable WinHTTP session.
pub new(a alc.Allocator, options Options) !$Client:
    impl := try impl_http.openClient(a, options.userAgent, options.connectTimeoutMs, options.sendTimeoutMs, options.receiveTimeoutMs, options.automaticDecompression)
    c Client
    c.impl = impl
    c.allocator = a
    ret c
..

destr Client.close() void:
    impl_http.closeClient(addrof this.impl)
..

Response(
    impl impl_http.Response
)

Response.statusCode() u16:
    ret this.impl.statusCode
..

# Raw response headers, including the HTTP status line, encoded as UTF-8.
# The returned string is borrowed from the Response.
Response.rawHeaders() str:
    ret this.impl.rawHeaders
..

# Returns a pull reader backed directly by WinHttpReadData.
# The Response must remain alive and unmoved while the reader is in use.
Response.body() !reader.Reader:
    ret try impl_http.responseBody(addrof this.impl)
..

destr Response.close() void:
    impl_http.closeResponse(addrof this.impl)
..

# Sends request headers and streams the request body before returning as soon
# as response headers are available. The response body is not buffered.
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
Client.get(url str) !$Response:
    headers Header[] = slices.fromPtr(none, 0)
    request := Request(method="GET", url=url, headers=headers)
    ret try this.send(request, noBody())
..

Client.post(url str, requestBody Body) !$Response:
    headers Header[] = slices.fromPtr(none, 0)
    request := Request(method="POST", url=url, headers=headers)
    ret try this.send(request, requestBody)
..
