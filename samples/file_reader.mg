mod main

use "../std/heap.mg"      as heap
use "../std/io.mg"        as io
use "../std/file.mg"      as file
use "../std/buffered.mg"  as buff
use "../std/errors.mg"    as err
use "../std/strings.mg"   as strs

main(args str[]) !void:
    a := heap.allocator()

    stdout := try io.stdout()
    stdin :=  try io.stdin()

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
        defer input.free()

        f := try file.open(input, file.mode().read())
        defer f.close()

        source := try f.reader()
        size := try f.count()
        contents := try source.read(size)
        defer contents.free()

        try out.write(contents)
        try out.writeLn("<EOF>")
    ..
.. 

