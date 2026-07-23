mod http_impl_win
# Windows HTTP backend used by the portable http module.


use "std:c" c
link "winhttp"

use "std:allocator" alc
use "std:reader"    reader
use "std:strings"   strings
use "std:slices"    slices
use "std:utf8"      utf8
use "std:cast"      cast
use "std:errors"    errors

ext ext_WinHttpOpen               WinHttpOpen(agent c.unsigned_short*, accessType c.unsigned_int, proxy c.unsigned_short*, bypass c.unsigned_short*, flags c.unsigned_int) ptr
ext ext_WinHttpConnect            WinHttpConnect(session ptr, server c.unsigned_short*, port c.unsigned_short, reserved c.unsigned_int) ptr
ext ext_WinHttpOpenRequest        WinHttpOpenRequest(connect ptr, verb c.unsigned_short*, object c.unsigned_short*, version c.unsigned_short*, referer c.unsigned_short*, acceptTypes ptr, flags c.unsigned_int) ptr
ext ext_WinHttpAddRequestHeaders  WinHttpAddRequestHeaders(request ptr, headers c.unsigned_short*, length c.unsigned_int, modifiers c.unsigned_int) c.unsigned_int
ext ext_WinHttpSendRequest        WinHttpSendRequest(request ptr, headers c.unsigned_short*, headerLength c.unsigned_int, optional ptr, optionalLength c.unsigned_int, totalLength c.unsigned_int, context c.uintptr_t) c.unsigned_int
ext ext_WinHttpWriteData          WinHttpWriteData(request ptr, buffer ptr, bytes c.unsigned_int, written c.unsigned_int*) c.unsigned_int
ext ext_WinHttpReceiveResponse    WinHttpReceiveResponse(request ptr, reserved ptr) c.unsigned_int
ext ext_WinHttpQueryHeaders       WinHttpQueryHeaders(request ptr, infoLevel c.unsigned_int, name c.unsigned_short*, buffer ptr, bufferLength c.unsigned_int*, index c.unsigned_int*) c.unsigned_int
ext ext_WinHttpQueryDataAvailable WinHttpQueryDataAvailable(request ptr, available c.unsigned_int*) c.unsigned_int
ext ext_WinHttpReadData           WinHttpReadData(request ptr, buffer ptr, bytes c.unsigned_int, read c.unsigned_int*) c.unsigned_int
ext ext_WinHttpSetTimeouts        WinHttpSetTimeouts(session ptr, resolve c.int, connect c.int, sendTimeout c.int, receive c.int) c.unsigned_int
ext ext_WinHttpSetOption          WinHttpSetOption(handle ptr, option c.unsigned_int, buffer ptr, length c.unsigned_int) c.unsigned_int
ext ext_WinHttpCloseHandle        WinHttpCloseHandle(handle ptr) c.unsigned_int
ext ext_WinHttpCrackUrl           WinHttpCrackUrl(url c.unsigned_short*, length c.unsigned_int, flags c.unsigned_int, components ptr) c.unsigned_int
ext ext_GetLastError              GetLastError() c.unsigned_int

URLComponents(
    structSize u32
    scheme u16*
    schemeLength u32
    schemeKind u32
    host u16*
    hostLength u32
    # INTERNET_PORT is u16 followed by native padding on 64-bit Windows.
    port u16
    user u16*
    userLength u32
    password u16*
    passwordLength u32
    path u16*
    pathLength u32
    extra u16*
    extraLength u32
)

pub Client(
    session ptr
    allocator alc.Allocator
)

pub Response(
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
    session ptr = ext_WinHttpOpen(slices.toPtr(agent), 4, none, none, 0)
    slices.free(a, agent)

    if session == none:
        throw fail("WinHttpOpen failed")
    ..

    ok u32 = ext_WinHttpSetTimeouts(session, 0, connectMs, sendMs, receiveMs)
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

    ret Client(session=session, allocator=a)
..

destr Client.close() void:
    if this.session != none:
        ext_WinHttpCloseHandle(this.session)
        this.session = none
    ..
..

pub closeClient(client Client*) void:
    client.close()
..

copyWideNT(a alc.Allocator, source u16*, count u64) !$u16[]:
    if count == 0 - 1:
        throw errors.wouldOverflow("wide string allocation size overflow")
    ..
    out u16* = try a.allocT[u16](count + 1)
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
        ret try utf8.utf8To16NT(a, "/") # " fix for bad syntax highlighting, nothing to see here
    ..
    if total == 0 - 1:
        throw errors.wouldOverflow("URL object name allocation size overflow")
    ..
    out u16* = try a.allocT[u16](total + 1)
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
    buffer := array u8[16384]
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
    ok u32 = ext_WinHttpQueryHeaders(request, 0x20000013, none, addrof status, addrof size, none)
    if ok == 0:
        throw fail("WinHttpQueryHeaders status failed")
    ..
    ret cast.u64to16(cast.u32to64(status))
..

queryRawHeaders(a alc.Allocator, request ptr) !$str:
    byteCount u32 = 0
    ext_WinHttpQueryHeaders(request, 22, none, none, addrof byteCount, none)
    if byteCount == 0:
        throw fail("WinHttpQueryHeaders size failed")
    ..
    wide u16* = try a.alloc(cast.u32to64(byteCount))
    ok u32 = ext_WinHttpQueryHeaders(request, 22, none, wide, addrof byteCount, none)
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
    if this.session == none:
        throw errors.invalidArgument("HTTP client is closed")
    ..
    if hasBody && bodyLength > 0xFFFFFFFF:
        throw errors.wouldOverflow("WinHTTP request bodies above 4 GiB are not implemented")
    ..

    url16 u16[] = try utf8.utf8To16NT(this.allocator, url)
    parts := URLComponents(
        structSize=cast.u64to32(sizeof URLComponents),
        scheme=none,
        schemeLength=0xFFFFFFFF,
        schemeKind=0,
        host=none,
        hostLength=0xFFFFFFFF,
        port=0,
        user=none,
        userLength=0,
        password=none,
        passwordLength=0,
        path=none,
        pathLength=0xFFFFFFFF,
        extra=none,
        extraLength=0xFFFFFFFF,
    )

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
    serverPort u16 = parts.port
    connection ptr = ext_WinHttpConnect(this.session, slices.toPtr(host), serverPort, 0)

    if connection == none:
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

    request ptr = ext_WinHttpOpenRequest(connection, slices.toPtr(verb), slices.toPtr(object), none, none, none, flags)
    slices.free(this.allocator, verb)
    slices.free(this.allocator, object)
    slices.free(this.allocator, host)
    slices.free(this.allocator, url16)

    if request == none:
        ext_WinHttpCloseHandle(connection)
        throw fail("WinHttpOpenRequest failed")
    ..

    if strings.countBytes(headers) > 0:
        added bool, addHeadersErr error = addHeaders(this.allocator, request, headers)
        if addHeadersErr.nok():
            ext_WinHttpCloseHandle(request)
            ext_WinHttpCloseHandle(connection)
            throw addHeadersErr
        ..
    ..

    total u32 = 0
    if hasBody:
        total = cast.u64to32(bodyLength)
    ..

    ok = ext_WinHttpSendRequest(request, none, 0, none, 0, total, 0)
    if ok == 0:
        ext_WinHttpCloseHandle(request)
        ext_WinHttpCloseHandle(connection)
        throw fail("WinHttpSendRequest failed")
    ..

    if hasBody:
        writtenBody u64, bodyErr error = writeBody(request, source, bodyLength)
        if bodyErr.nok():
            ext_WinHttpCloseHandle(request)
            ext_WinHttpCloseHandle(connection)
            throw bodyErr
        ..
    ..

    ok = ext_WinHttpReceiveResponse(request, none)
    if ok == 0:
        ext_WinHttpCloseHandle(request)
        ext_WinHttpCloseHandle(connection)
        throw fail("WinHttpReceiveResponse failed")
    ..

    status u16, statusErr error = queryStatus(request)
    if statusErr.nok():
        ext_WinHttpCloseHandle(request)
        ext_WinHttpCloseHandle(connection)
        throw statusErr
    ..

    raw str, headerErr error = queryRawHeaders(this.allocator, request)
    if headerErr.nok():
        ext_WinHttpCloseHandle(request)
        ext_WinHttpCloseHandle(connection)
        throw headerErr
    ..

    ret Response(
        connection=connection,
        request=request,
        allocator=this.allocator,
        rawHeaders=raw,
        statusCode=status,
        eof=false,
        open=true,
    )
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
        this.request = none
        this.connection = none
    ..
..

pub closeResponse(response Response*) void:
    response.close()
..
