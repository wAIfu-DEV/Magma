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
    if body.isPresent():
        ret
    ..
    headers http.Header[] = slices.fromPtr(none, 0)
    request := http.Request(method="GET", url="https://example.com/", headers=headers)
..
