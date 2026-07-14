mod main

use "../std_checked/heap.mg"      heap
use "../std_checked/io.mg"        io
use "../std_checked/file.mg"      file
use "../std_checked/buffered.mg"  buff
use "../std_checked/errors.mg"    err
use "../std_checked/strings.mg"   strs

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

        reader := buff.readerBuffered(a, f.reader())
        defer reader.close()

        while true:
            line str, e error = reader.readLn(a)

            if err.is(e, err.endOfFile("")):
                out.writeLn("<EOF>")
                break
            else:
                throw e
            ..
            
            out.writeLn(line)
            strs.free(a, line)
        ..
    ..
.. 

