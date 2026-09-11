mod main

use "../std/heap.mg"      as heap
use "../std/io.mg"        as io
use "../std/fmt.mg"       as fmt
use "../std/strings.mg"   as strs
use "../std/http.mg"      as http
use "../std/slices.mg"    as slices

pub main(args str[]) !void:
    a := heap.allocator()

    in :=  try io.stdin()
    defer in.close()

    io.printLn("Started program. URL to query.")

    client := try http.new(http.defaultOptions())
    defer client.close()

    loop true:
        io.print("URL: ")

        input := try in.readLn(a)
        defer input.free()

        headers http.Header[] = slices.fromPtr(none, 0)
        request := http.noBody("GET", input, headers)
        resp := try client.send(request)
        defer resp.close()

        if resp.statusCode != 200:
            fmt.str(a, "Request failed with code: ").int(resp.statusCode).str("\n").print()
            continue
        ..

        io.printLn(resp.body)
        io.printLn("<END OF RESPONSE>")
    ..
.. 
