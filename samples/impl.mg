mod main

use "../std/heap.mg"      heap
use "../std/io.mg"        io
use "../std/strings.mg"   stg
use "../std/file.mg"      file
use "../std/buffered.mg"  buffered
use "../std/errors.mg"    errors

pub main(args str[]) !void:
    a := heap.allocator()

    stdout := io.stdout(a)
    defer stdout.close()

    stdin := io.stdin(a)
    defer stdin.close()

    out := stdout.writer()

    out.writeLn("Started program. Write file path to print.")

    while true:
        out.write("Path: ")
        stdout.flush()

        input := stdin.readLn(a)

        f := try file.open(a, input, file.modeRead())
        defer f.close()

        reader := buffered.readerBuffered(a, f.reader())

        while true:
            line str, err error = reader.readLn(a)
            if errors.is(err, errors.errEndOfFile("")):
                out.writeLn("<END OF FILE>")
                break
            else:
                throw err
            ..
            out.writeLn(line)
        ..
    ..
.. 

