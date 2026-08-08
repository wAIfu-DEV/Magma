mod main

use "../std/heap.mg"      heap
use "../std/io.mg"        io
use "../std/fmt.mg"       fmt
use "../std/strings.mg"   strs
use "../std/http.mg"      http
use "../std/slices.mg"    slices

pub main(args str[]) !void:
    a := heap.allocator()

    in :=  try io.stdin(a)

    defer:
        in.close()
    ..

    io.printLn("Started program. URL to query.")

    client := try http.new(a, http.defaultOptions())
    defer client.close()

    loop true:
        io.print("URL: ")

        input := try in.readLn(a)
        defer input.free(a)

        headers http.Header[] = slices.fromPtr(none, 0)
        request := http.noBody("GET", input, headers)
        resp := try client.send(request)
        defer resp.close()

        if resp.statusCode != 200:
            fmt.str(a, "Request failed with code: ").int(resp.statusCode).print()
            io.printLn("")
            continue
        ..

        io.printLn(resp.body)
        io.printLn("<END OF RESPONSE>")
    ..
.. 
