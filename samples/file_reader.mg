mod main

use "../std/heap.mg"      heap
use "../std/io.mg"        io
use "../std/file.mg"      file
use "../std/buffered.mg"  buff
use "../std/errors.mg"    err
use "../std/strings.mg"   strs

main(args str[]) !void:
    a := heap.allocator()

    stdout := try io.stdout(a)
    stdin :=  try io.stdin(a)

    defer:
        stdout.close()
        stdin.close()
    ..

    out := stdout.writer()
    out.writeLn("Started program. Write file path to print.")

    while true:
        out.write("Path: ")
        stdout.flush()

        input := stdin.readLn(a)
        defer strs.free(a, input)

        f := try file.open(a, input, file.mode().read())
        defer f.close()

        contents := f.reader().read(a, f.count())
        defer strs.free(a, contents)

        out.write(contents)
        out.writeLn("<EOF>")
    ..
.. 

