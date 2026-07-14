mod main

use "../http.mg" http
use "../cast.mg" cast
use "../slices.mg" slices

pub main() void:
    options := http.defaultOptions()
    if options.connectTimeoutMs == 0:
        ret
    ..
    body := http.noBody()
    if body.present:
        ret
    ..
    headers http.Header[] = slices.fromPtr(cast.utop(0), 0)
    request http.Request
    request.method = "GET"
    request.url = "https://example.com/"
    request.headers = headers
..
