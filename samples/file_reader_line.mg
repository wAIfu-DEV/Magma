mod main

use "../std/heap.mg"      heap
use "../std/io.mg"        io
use "../std/file.mg"      file
use "../std/buffered.mg"  buff
use "../std/errors.mg"    err
use "../std/strings.mg"   strs

main(args str[]) !void:
    a := heap.allocator()
    stdin :=  try io.stdin()
    stdout := try io.stdout()

    defer:
        stdin.close()
        stdout.close()
    ..
    out := stdout.writer()

    try out.writeLn("Started program. Write file path to print.")

    loop true:
        try out.write("Path: ")
        try stdout.flush()

        input := try stdin.readLn(a)
        defer input.free(a)

        f := try file.open(input, file.mode().read())
        defer f.close()

        source := try f.reader()
        reader := try buff.readerBuffered(a, source)
        defer reader.close()

        loop true:
            line, e := reader.readLn(a)

            if e.nok():
                if e.code() == 4:
                    try out.writeLn("<EOF>")
                    break
                ..
                throw e
            ..
            
            try out.writeLn(line)
            line.free(a)
        ..
    ..
.. 

