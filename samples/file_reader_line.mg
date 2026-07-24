mod main

use "../std/heap.mg"      heap
use "../std/io.mg"        io
use "../std/file.mg"      file
use "../std/buffered.mg"  buff
use "../std/errors.mg"    err
use "../std/strings.mg"   strs

main(args str[]) !void:
    a := heap.allocator()
    stdin :=  try io.stdin(a)

    defer:
        stdin.close()
    ..

    io.writeLn("Started program. Write file path to print.")

    while true:
        io.write("Path: ")
        io.flush()

        input := stdin.readLn(a)
        defer input.free(a)

        f := try file.open(a, input, file.mode().read())
        defer f.close()

        reader := buff.readerBuffered(a, f.reader())
        defer reader.close()

        while true:
            line, e := reader.readLn(a)

            if e.nok():
                if e.code() == 4:
                    io.writeLn("<EOF>")
                    break
                ..
                throw e
            ..
            
            io.writeLn(line)
            line.free(a)
        ..
    ..
.. 

