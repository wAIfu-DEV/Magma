mod main

use "../std/heap.mg"      heap
use "../std/io.mg"        io
use "../std/strings.mg"   strs
use "../std/http.mg"      http
use "../std/builder.mg"   builder
use "../std/thread_pool.mg" thread_pool

pub main(args str[]) !void:
    a := heap.allocator()

    tp := try thread_pool.newDefault(a)
    defer tp.close()

    out := try io.stdout(a)
    in :=  try io.stdin(a)

    defer:
        out.close()
        in.close()
    ..

    out.writeLn("Started program. URL to query.")

    client := try http.new(a, http.defaultOptions())
    defer client.close()

    while true:
        out.write("URL: ")
        out.flush()

        input := try in.readLn(a)
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
            readFuture := try body.readAsync(tp, a, 512)
            chunk := try readFuture.await()

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
