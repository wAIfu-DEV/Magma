mod main

use "../std/heap.mg"      heap
use "../std/io.mg"        io
use "../std/fmt.mg"       fmt
use "../std/strings.mg"   strs
use "../std/http.mg"      http
use "../std/builder.mg"   builder
use "../std/thread_pool.mg" thread_pool
use "../std/async.mg"       async

pub main(args str[]) !void:
    a := heap.allocator()

    tp := try thread_pool.newDefault(a)
    defer tp.close()
    
    as := async.new(tp, a)

    in :=  try io.stdin(a)

    defer:
        in.close()
    ..

    io.printLn("Started program. URL to query.")

    client := try http.new(a, http.defaultOptions())
    defer client.close()

    while true:
        io.print("URL: ")

        input := try in.readLn(a)
        defer input.free(a)

        resp := try client.get(input)
        defer resp.close()

        if resp.statusCode() != 200:
            fmt.str(a, "Request failed with code: ").int(resp.statusCode()).print()
            io.printLn("")
            continue
        ..

        body := resp.body()

        bld := builder.new(a)
        defer bld.free()

        while true:
            readFuture := try as.read(body, 512)
            chunk := try readFuture.await()

            if chunk.countBytes() == 0:
                res := try bld.build()
                defer res.free(a)

                io.printLn(res)
                io.printLn("<END OF RESPONSE>")
                break
            ..

            try bld.appendOwned(chunk)
        ..
    ..
.. 
