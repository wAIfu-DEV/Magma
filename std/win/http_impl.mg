mod http_impl_win

link "winhttp"

use "../allocator.mg" alc
use "../reader.mg"    reader
use "../strings.mg"   strings
use "../slices.mg"    slices
use "../utf8.mg"      utf8
use "../cast.mg"      cast
use "../errors.mg"    errors

ext ext_WinHttpOpen               WinHttpOpen(agent u16*, accessType u32, proxy u16*, bypass u16*, flags u32) ptr
ext ext_WinHttpConnect            WinHttpConnect(session ptr, server u16*, port u16, reserved u32) ptr
ext ext_WinHttpOpenRequest        WinHttpOpenRequest(connect ptr, verb u16*, object u16*, version u16*, referer u16*, acceptTypes ptr, flags u32) ptr
ext ext_WinHttpAddRequestHeaders  WinHttpAddRequestHeaders(request ptr, headers u16*, length u32, modifiers u32) u32
ext ext_WinHttpSendRequest        WinHttpSendRequest(request ptr, headers u16*, headerLength u32, optional ptr, optionalLength u32, totalLength u32, context u64) u32
ext ext_WinHttpWriteData          WinHttpWriteData(request ptr, buffer ptr, bytes u32, written u32*) u32
ext ext_WinHttpReceiveResponse    WinHttpReceiveResponse(request ptr, reserved ptr) u32
ext ext_WinHttpQueryHeaders       WinHttpQueryHeaders(request ptr, infoLevel u32, name u16*, buffer ptr, bufferLength u32*, index u32*) u32
ext ext_WinHttpQueryDataAvailable WinHttpQueryDataAvailable(request ptr, available u32*) u32
ext ext_WinHttpReadData           WinHttpReadData(request ptr, buffer ptr, bytes u32, read u32*) u32
ext ext_WinHttpSetTimeouts        WinHttpSetTimeouts(session ptr, resolve i32, connect i32, send i32, receive i32) u32
ext ext_WinHttpSetOption          WinHttpSetOption(handle ptr, option u32, buffer ptr, length u32) u32
ext ext_WinHttpCloseHandle        WinHttpCloseHandle(handle ptr) u32
ext ext_WinHttpCrackUrl           WinHttpCrackUrl(url u16*, length u32, flags u32, components ptr) u32
ext ext_GetLastError              GetLastError() u32

URLComponents(
    structSize u32
    scheme u16*
    schemeLength u32
    schemeKind u32
    host u16*
    hostLength u32
    # INTERNET_PORT is u16 followed by padding on 64-bit Windows. Using u32
    # preserves the native offset while avoiding the current i16 field-load
    # lowering bug; only the low 16 bits are consumed below.
    port u32
    user u16*
    userLength u32
    password u16*
    passwordLength u32
    path u16*
    pathLength u32
    extra u16*
    extraLength u32
)

Client(
    session ptr
    allocator alc.Allocator
    open bool
)

Response(
    connection ptr
    request ptr
    allocator alc.Allocator
    rawHeaders $str
    statusCode u16
    eof bool
    open bool
)

fail(message str) error:
    ret errors.native(ext_GetLastError(), message)
..

pub openClient(a alc.Allocator, userAgent str, connectMs u32, sendMs u32, receiveMs u32, decompress bool) !$Client:
    agent u16[] = try utf8.utf8To16NT(a, userAgent)
    session ptr = ext_WinHttpOpen(slices.toPtr(agent), 4, cast.utop(0), cast.utop(0), 0)
    slices.free(a, agent)

    if cast.ptou(session) == 0:
        throw fail("WinHttpOpen failed")
    ..

    ok u32 = ext_WinHttpSetTimeouts(session, 0, cast.u64to32(cast.u32to64(connectMs)), cast.u64to32(cast.u32to64(sendMs)), cast.u64to32(cast.u32to64(receiveMs)))
    if ok == 0:
        ext_WinHttpCloseHandle(session)
        throw fail("WinHttpSetTimeouts failed")
    ..

    if decompress:
        decompression u32 = 3
        ok = ext_WinHttpSetOption(session, 118, addrof decompression, 4)
        if ok == 0:
            ext_WinHttpCloseHandle(session)
            throw fail("WinHttpSetOption decompression failed")
        ..
    ..

    c Client
    c.session = session
    c.allocator = a
    c.open = true
    ret c
..

destr Client.close() void:
    if this.open:
        ext_WinHttpCloseHandle(this.session)
        this.open = false
        this.session = cast.utop(0)
    ..
..

pub closeClient(client Client*) void:
    client.close()
..

copyWideNT(a alc.Allocator, source u16*, count u64) !$u16[]:
    out u16* = try a.alloc((count + 1) * sizeof u16)
    i u64 = 0
    while i < count:
        out[i] = source[i]
        i = i + 1
    ..
    out[count] = 0
    ret slices.fromPtr(out, count + 1)
..

makeObjectName(a alc.Allocator, parts URLComponents*) !$u16[]:
    total u64 = cast.u32to64(parts.pathLength) + cast.u32to64(parts.extraLength)
    if total == 0:
        ret try utf8.utf8To16NT(a, "/")
    ..
    out u16* = try a.alloc((total + 1) * sizeof u16)
    pathPtr u16* = parts.path
    extraPtr u16* = parts.extra
    i u64 = 0
    while i < cast.u32to64(parts.pathLength):
        out[i] = pathPtr[i]
        i = i + 1
    ..
    j u64 = 0
    while j < cast.u32to64(parts.extraLength):
        out[i + j] = extraPtr[j]
        j = j + 1
    ..
    out[total] = 0
    ret slices.fromPtr(out, total + 1)
..

addHeaders(a alc.Allocator, request ptr, headers str) !bool:
    headers16 u16[] = try utf8.utf8To16(a, headers)
    total u64 = slices.count(headers16)
    if total > 0xFFFFFFFF:
        slices.free(a, headers16)
        throw errors.wouldOverflow("HTTP header is too large")
    ..
    ok u32 = ext_WinHttpAddRequestHeaders(request, slices.toPtr(headers16), cast.u64to32(total), 0xA0000000)
    slices.free(a, headers16)
    if ok == 0:
        throw fail("WinHttpAddRequestHeaders failed")
    ..
    ret true
..

writeBody(request ptr, source reader.Reader, length u64) !u64:
    remaining u64 = length
    buffer u8[16384]
    while remaining > 0:
        wanted u64 = remaining
        if wanted > 16384:
            wanted = 16384
        ..
        count u64 = try source.readToBuff(buffer, wanted)
        if count == 0:
            throw errors.failure("HTTP request body ended before its declared length")
        ..
        offset u64 = 0
        while offset < count:
            written u32 = 0
            next ptr = cast.utop(cast.ptou(slices.toPtr(buffer)) + offset)
            ok u32 = ext_WinHttpWriteData(request, next, cast.u64to32(count - offset), addrof written)
            if ok == 0:
                throw fail("WinHttpWriteData failed")
            ..
            if written == 0:
                throw errors.failure("WinHttpWriteData made no progress")
            ..
            offset = offset + cast.u32to64(written)
        ..
        remaining = remaining - count
    ..
    ret length
..

queryStatus(request ptr) !u16:
    status u32 = 0
    size u32 = 4
    ok u32 = ext_WinHttpQueryHeaders(request, 0x20000013, cast.utop(0), addrof status, addrof size, cast.utop(0))
    if ok == 0:
        throw fail("WinHttpQueryHeaders status failed")
    ..
    ret cast.u64to16(cast.u32to64(status))
..

queryRawHeaders(a alc.Allocator, request ptr) !$str:
    byteCount u32 = 0
    ext_WinHttpQueryHeaders(request, 22, cast.utop(0), cast.utop(0), addrof byteCount, cast.utop(0))
    if byteCount == 0:
        throw fail("WinHttpQueryHeaders size failed")
    ..
    wide u16* = try a.alloc(cast.u32to64(byteCount))
    ok u32 = ext_WinHttpQueryHeaders(request, 22, cast.utop(0), wide, addrof byteCount, cast.utop(0))
    if ok == 0:
        a.free(wide)
        throw fail("WinHttpQueryHeaders failed")
    ..
    units u64 = cast.u32to64(byteCount) / sizeof u16
    if units > 0 && wide[units - 1] == 0:
        units = units - 1
    ..
    view u16[] = slices.fromPtr(wide, units)
    result str = try utf8.utf16to8(a, view)
    a.free(wide)
    ret result
..

pub Client.send(method str, url str, headers str, source reader.Reader, bodyLength u64, hasBody bool) !$Response:
    if this.open == false:
        throw errors.invalidArgument("HTTP client is closed")
    ..
    if hasBody && bodyLength > 0xFFFFFFFF:
        throw errors.wouldOverflow("WinHTTP request bodies above 4 GiB are not implemented")
    ..

    url16 u16[] = try utf8.utf8To16NT(this.allocator, url)
    parts URLComponents
    parts.structSize = cast.u64to32(sizeof URLComponents)
    parts.schemeLength = 0xFFFFFFFF
    parts.hostLength = 0xFFFFFFFF
    parts.pathLength = 0xFFFFFFFF
    parts.extraLength = 0xFFFFFFFF
    ok u32 = ext_WinHttpCrackUrl(slices.toPtr(url16), 0, 0, addrof parts)
    if ok == 0:
        slices.free(this.allocator, url16)
        throw fail("WinHttpCrackUrl failed")
    ..
    if parts.schemeKind != 1 && parts.schemeKind != 2:
        slices.free(this.allocator, url16)
        throw errors.invalidArgument("HTTP URL must use http or https")
    ..

    host u16[] = try copyWideNT(this.allocator, parts.host, cast.u32to64(parts.hostLength))
    object u16[] = try makeObjectName(this.allocator, addrof parts)
    verb u16[] = try utf8.utf8To16NT(this.allocator, method)
    serverPort u16 = cast.u64to16(cast.u32to64(parts.port))
    connection ptr = ext_WinHttpConnect(this.session, slices.toPtr(host), serverPort, 0)
    if cast.ptou(connection) == 0:
        slices.free(this.allocator, verb)
        slices.free(this.allocator, object)
        slices.free(this.allocator, host)
        slices.free(this.allocator, url16)
        throw fail("WinHttpConnect failed")
    ..
    flags u32 = 0
    if parts.schemeKind == 2:
        flags = 0x00800000
    ..
    request ptr = ext_WinHttpOpenRequest(connection, slices.toPtr(verb), slices.toPtr(object), cast.utop(0), cast.utop(0), cast.utop(0), flags)
    slices.free(this.allocator, verb)
    slices.free(this.allocator, object)
    slices.free(this.allocator, host)
    slices.free(this.allocator, url16)
    if cast.ptou(request) == 0:
        ext_WinHttpCloseHandle(connection)
        throw fail("WinHttpOpenRequest failed")
    ..

    if strings.countBytes(headers) > 0:
        added bool, addHeadersErr error = addHeaders(this.allocator, request, headers)
        if errors.code(addHeadersErr) != 0:
            ext_WinHttpCloseHandle(request)
            ext_WinHttpCloseHandle(connection)
            throw addHeadersErr
        ..
    ..

    total u32 = 0
    if hasBody:
        total = cast.u64to32(bodyLength)
    ..
    ok = ext_WinHttpSendRequest(request, cast.utop(0), 0, cast.utop(0), 0, total, 0)
    if ok == 0:
        ext_WinHttpCloseHandle(request)
        ext_WinHttpCloseHandle(connection)
        throw fail("WinHttpSendRequest failed")
    ..
    if hasBody:
        writtenBody u64, bodyErr error = writeBody(request, source, bodyLength)
        if errors.code(bodyErr) != 0:
            ext_WinHttpCloseHandle(request)
            ext_WinHttpCloseHandle(connection)
            throw bodyErr
        ..
    ..
    ok = ext_WinHttpReceiveResponse(request, cast.utop(0))
    if ok == 0:
        ext_WinHttpCloseHandle(request)
        ext_WinHttpCloseHandle(connection)
        throw fail("WinHttpReceiveResponse failed")
    ..

    status u16, statusErr error = queryStatus(request)
    if errors.code(statusErr) != 0:
        ext_WinHttpCloseHandle(request)
        ext_WinHttpCloseHandle(connection)
        throw statusErr
    ..
    raw str, headerErr error = queryRawHeaders(this.allocator, request)
    if errors.code(headerErr) != 0:
        ext_WinHttpCloseHandle(request)
        ext_WinHttpCloseHandle(connection)
        throw headerErr
    ..
    response Response
    response.connection = connection
    response.request = request
    response.allocator = this.allocator
    response.rawHeaders = raw
    response.statusCode = status
    response.eof = false
    response.open = true
    ret response
..

pub send(client Client*, method str, url str, headers str, source reader.Reader, bodyLength u64, hasBody bool) !$Response:
    ret try client.send(method, url, headers, source, bodyLength, hasBody)
..

readResponse(response Response*, buffer u8[], count u64) !u64:
    if response.open == false:
        throw errors.invalidArgument("read from closed HTTP response")
    ..
    if response.eof || count == 0:
        ret 0
    ..
    wanted u64 = count
    if wanted > 0xFFFFFFFF:
        wanted = 0xFFFFFFFF
    ..
    available u32 = 0
    ok u32 = ext_WinHttpQueryDataAvailable(response.request, addrof available)
    if ok == 0:
        throw fail("WinHttpQueryDataAvailable failed")
    ..
    if available > 0 && cast.u32to64(available) < wanted:
        wanted = cast.u32to64(available)
    ..
    read u32 = 0
    ok = ext_WinHttpReadData(response.request, slices.toPtr(buffer), cast.u64to32(wanted), addrof read)
    if ok == 0:
        throw fail("WinHttpReadData failed")
    ..
    if read == 0:
        response.eof = true
    ..
    ret cast.u32to64(read)
..

Response.body() !reader.Reader:
    if this.open == false:
        throw errors.invalidArgument("HTTP response is closed")
    ..
    ret reader.new(this, readResponse)
..

pub responseBody(response Response*) !reader.Reader:
    ret try response.body()
..

destr Response.close() void:
    if this.open:
        ext_WinHttpCloseHandle(this.request)
        ext_WinHttpCloseHandle(this.connection)
        strings.free(this.allocator, this.rawHeaders)
        this.open = false
        this.eof = true
        this.request = cast.utop(0)
        this.connection = cast.utop(0)
    ..
..

pub closeResponse(response Response*) void:
    response.close()
..
