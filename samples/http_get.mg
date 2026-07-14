mod main

use "../std/heap.mg"      heap
use "../std/io.mg"        io
use "../std/file.mg"      file
use "../std/buffered.mg"  buff
use "../std/errors.mg"    err
use "../std/strings.mg"   strs
use "../std/http.mg"      http
use "../std/builder.mg"   builder

main(args str[]) !void:
    a := heap.allocator()

    stdout := try io.stdout(a)
    stdin :=  try io.stdin(a)

    defer:
        stdout.close()
        stdin.close()
    ..

    out := stdout.writer()
    out.writeLn("Started program. URL to query.")

    client := try http.new(a, http.defaultOptions())
    defer client.close()

    while true:
        out.write("URL: ")
        stdout.flush()

        input := stdin.readLn(a)
        defer strs.free(a, input)

        resp := try client.get(input)
        defer resp.close()

        if resp.statusCode() != 200:
            out.write("Request failed with code: ")
            out.writeInt64(resp.statusCode())
            out.writeLn("")
            continue
        ..

        body := resp.body()
        
        bld := builder.new(a)
        defer bld.free()

        while true:
            chunk := try body.read(a, 512)

            if strs.countBytes(chunk) == 0:
                res := try bld.build()
                defer strs.free(a, res)

                out.writeLn(res)
                out.writeLn("<END OF RESPONSE>")
                break
            ..

            try bld.appendOwned(chunk)
        ..
    ..
.. 

