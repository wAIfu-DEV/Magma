mod http

use "allocator.mg" alc
use "reader.mg"    reader
use "cast.mg"      cast
use "builder.mg"   builder
use "strings.mg"   strings
use "slices.mg"    slices
use "errors.mg"    errors

@platform("windows")
use "win/http_impl.mg" impl_http

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
    present bool
)

pub noBody() Body:
    b Body
    b.present = false
    ret b
..

pub body(source reader.Reader, length u64) Body:
    b Body
    b.source = source
    b.length = length
    b.present = true
    ret b
..

Options(
    userAgent str
    connectTimeoutMs u32
    sendTimeoutMs u32
    receiveTimeoutMs u32
    automaticDecompression bool
)

pub defaultOptions() Options:
    o Options
    o.userAgent = "Magma/0"
    o.connectTimeoutMs = 30000
    o.sendTimeoutMs = 30000
    o.receiveTimeoutMs = 30000
    o.automaticDecompression = true
    ret o
..

Client(
    impl impl_http.Client
    allocator alc.Allocator
)

# Opens a reusable WinHTTP session.
pub new(a alc.Allocator, options Options) !$Client:
    c Client
    c.impl = try impl_http.openClient(a, options.userAgent, options.connectTimeoutMs, options.sendTimeoutMs, options.receiveTimeoutMs, options.automaticDecompression)
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
Client.send(request Request, body Body) !$Response:
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
    responseImpl impl_http.Response, sendErr error = impl_http.send(addrof this.impl, request.method, request.url, rawHeaders, body.source, body.length, body.present)
    strings.free(this.allocator, rawHeaders)
    if errors.code(sendErr) != 0:
        drop[Response](response)
        drop[impl_http.Response](responseImpl)
        throw sendErr
    ..
    response.impl = responseImpl
    ret response
..

# Convenience GET. It still returns a streaming Response.
Client.get(url str) !$Response:
    empty Header[] = slices.fromPtr(cast.utop(0), 0)
    request Request
    request.method = "GET"
    request.url = url
    request.headers = empty
    ret try this.send(request, noBody())
..
