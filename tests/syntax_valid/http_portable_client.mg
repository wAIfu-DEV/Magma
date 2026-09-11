mod main

use "std:http" as http
use "std:heap" as heap
use "std:slices" as slices

pub main() !void:
    options := http.defaultOptions()
    client := try http.new(options)
    headers http.Header[] = slices.fromPtr(none, 0)
    request := http.noBody("GET", "http://127.0.0.1/", headers)
    client.close()
..
