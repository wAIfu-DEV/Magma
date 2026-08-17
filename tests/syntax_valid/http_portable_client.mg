mod main

use "std:http" http
use "std:heap" heap
use "std:slices" slices

pub main() !void:
    options := http.defaultOptions()
    client := try http.new(options)
    headers http.Header[] = slices.fromPtr(none, 0)
    request := http.noBody("GET", "http://127.0.0.1/", headers)
    client.close()
..
