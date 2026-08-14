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
    try out.writeLn("Started program. Write file path to print.")

    loop true:
        try out.write("Path: ")
        try stdout.flush()

        input := try stdin.readLn(a)
        defer input.free(a)

        f := try file.open(a, input, file.mode().read())
        defer f.close()

        source := try f.reader()
        size := try f.count()
        contents := try source.read(a, size)
        defer contents.free(a)

        try out.write(contents)
        try out.writeLn("<EOF>")
    ..
.. 

