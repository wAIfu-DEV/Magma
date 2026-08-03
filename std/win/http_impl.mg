mod http_impl_win
# Windows HTTP backend used by the portable http module.


use "std:win/types" win
link "winhttp"

use "std:allocator" alc
use "std:reader"    reader
use "std:strings"   strings
use "std:slices"    slices
use "std:utf8"      utf8
use "std:cast"      cast
use "std:errors"    errors

ext ext_WinHttpOpen               WinHttpOpen(agent win.LPCWSTR, accessType win.DWORD, proxy win.LPCWSTR, bypass win.LPCWSTR, flags win.DWORD) win.HANDLE
ext ext_WinHttpConnect            WinHttpConnect(session win.HANDLE, server win.LPCWSTR, port win.INTERNET_PORT, reserved win.DWORD) win.HANDLE
ext ext_WinHttpOpenRequest        WinHttpOpenRequest(connect win.HANDLE, verb win.LPCWSTR, object win.LPCWSTR, version win.LPCWSTR, referer win.LPCWSTR, acceptTypes win.LPCWSTR*, flags win.DWORD) win.HANDLE
ext ext_WinHttpAddRequestHeaders  WinHttpAddRequestHeaders(request win.HANDLE, headers win.LPCWSTR, length win.DWORD, modifiers win.DWORD) win.BOOL
ext ext_WinHttpSendRequest        WinHttpSendRequest(request win.HANDLE, headers win.LPCWSTR, headerLength win.DWORD, optional win.LPVOID, optionalLength win.DWORD, totalLength win.DWORD, context win.DWORD_PTR) win.BOOL
ext ext_WinHttpWriteData          WinHttpWriteData(request win.HANDLE, buffer win.LPCVOID, bytes win.DWORD, written win.DWORD*) win.BOOL
ext ext_WinHttpReceiveResponse    WinHttpReceiveResponse(request win.HANDLE, reserved win.LPVOID) win.BOOL
ext ext_WinHttpQueryHeaders       WinHttpQueryHeaders(request win.HANDLE, infoLevel win.DWORD, name win.LPCWSTR, buffer win.LPVOID, bufferLength win.DWORD*, index win.DWORD*) win.BOOL
ext ext_WinHttpQueryDataAvailable WinHttpQueryDataAvailable(request win.HANDLE, available win.DWORD*) win.BOOL
ext ext_WinHttpReadData           WinHttpReadData(request win.HANDLE, buffer win.LPVOID, bytes win.DWORD, read win.DWORD*) win.BOOL
ext ext_WinHttpSetTimeouts        WinHttpSetTimeouts(session win.HANDLE, resolve win.INT, connect win.INT, sendTimeout win.INT, receive win.INT) win.BOOL
ext ext_WinHttpSetOption          WinHttpSetOption(handle win.HANDLE, option win.DWORD, buffer win.LPVOID, length win.DWORD) win.BOOL
ext ext_WinHttpCloseHandle        WinHttpCloseHandle(handle win.HANDLE) win.BOOL
ext ext_WinHttpCrackUrl           WinHttpCrackUrl(url win.LPCWSTR, length win.DWORD, flags win.DWORD, components win.LPVOID) win.BOOL
ext ext_GetLastError              GetLastError() win.DWORD

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
    onerror ext_WinHttpCloseHandle(session)

    ok win.BOOL = ext_WinHttpSetTimeouts(session, 0, cast.u32toi32(connectMs), cast.u32toi32(sendMs), cast.u32toi32(receiveMs))
    if ok == 0:
        throw fail("WinHttpSetTimeouts failed")
    ..

    if decompress:
        decompression u32 = 3
        ok = ext_WinHttpSetOption(session, 118, addrof decompression, 4)
        if ok == 0:
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
    defer slices.free(a, headers16)
    total u64 = slices.count(headers16)
    if total > 0xFFFFFFFF:
        throw errors.wouldOverflow("HTTP header is too large")
    ..
    ok win.BOOL = ext_WinHttpAddRequestHeaders(request, slices.toPtr(headers16), cast.u64to32(total), 0xA0000000)
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
            ok win.BOOL = ext_WinHttpWriteData(request, next, cast.u64to32(count - offset), addrof written)
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
    ok win.BOOL = ext_WinHttpQueryHeaders(request, 0x20000013, none, addrof status, addrof size, none)
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
    defer a.free(wide)
    ok win.BOOL = ext_WinHttpQueryHeaders(request, 22, none, wide, addrof byteCount, none)
    if ok == 0:
        throw fail("WinHttpQueryHeaders failed")
    ..
    units u64 = cast.u32to64(byteCount) / sizeof u16
    if units > 0 && wide[units - 1] == 0:
        units = units - 1
    ..
    view u16[] = slices.fromPtr(wide, units)
    result str = try utf8.utf16to8(a, view)
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
    defer slices.free(this.allocator, url16)
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

    ok win.BOOL = ext_WinHttpCrackUrl(slices.toPtr(url16), 0, 0, addrof parts)
    if ok == 0:
        throw fail("WinHttpCrackUrl failed")
    ..
    
    if parts.schemeKind != 1 && parts.schemeKind != 2:
        throw errors.invalidArgument("HTTP URL must use http or https")
    ..

    host u16[] = try copyWideNT(this.allocator, parts.host, cast.u32to64(parts.hostLength))
    defer slices.free(this.allocator, host)
    object u16[] = try makeObjectName(this.allocator, addrof parts)
    defer slices.free(this.allocator, object)
    verb u16[] = try utf8.utf8To16NT(this.allocator, method)
    defer slices.free(this.allocator, verb)
    serverPort u16 = parts.port
    connection ptr = ext_WinHttpConnect(this.session, slices.toPtr(host), serverPort, 0)

    if connection == none:
        throw fail("WinHttpConnect failed")
    ..
    onerror ext_WinHttpCloseHandle(connection)

    flags u32 = 0
    if parts.schemeKind == 2:
        flags = 0x00800000
    ..

    request ptr = ext_WinHttpOpenRequest(connection, slices.toPtr(verb), slices.toPtr(object), none, none, none, flags)

    if request == none:
        throw fail("WinHttpOpenRequest failed")
    ..
    onerror ext_WinHttpCloseHandle(request)

    if headers.countBytes() > 0:
        added bool, addHeadersErr error = addHeaders(this.allocator, request, headers)
        if addHeadersErr.nok():
            throw addHeadersErr
        ..
    ..

    total u32 = 0
    if hasBody:
        total = cast.u64to32(bodyLength)
    ..

    ok = ext_WinHttpSendRequest(request, none, 0, none, 0, total, 0)
    if ok == 0:
        throw fail("WinHttpSendRequest failed")
    ..

    if hasBody:
        writtenBody u64, bodyErr error = writeBody(request, source, bodyLength)
        if bodyErr.nok():
            throw bodyErr
        ..
    ..

    ok = ext_WinHttpReceiveResponse(request, none)
    if ok == 0:
        throw fail("WinHttpReceiveResponse failed")
    ..

    status u16, statusErr error = queryStatus(request)
    if statusErr.nok():
        throw statusErr
    ..

    raw str, headerErr error = queryRawHeaders(this.allocator, request)
    if headerErr.nok():
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
    ok win.BOOL = ext_WinHttpQueryDataAvailable(response.request, addrof available)
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
        this.rawHeaders.free(this.allocator)
        this.open = false
        this.eof = true
        this.request = none
        this.connection = none
    ..
..

pub closeResponse(response Response*) void:
    response.close()
..
