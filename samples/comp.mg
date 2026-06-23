mod main

use "../std/heap.mg"      heap
use "../std/allocator.mg" alc
use "../std/io.mg"        io
use "../std/buffered.mg"  buf
use "../std/writer.mg"    wr
use "../std/reader.mg"    rd
use "../std/strings.mg"   stg

pub main(args str[]) !void:
    a alc.Allocator = heap.allocator()

    stdout buf.Writer = io.stdout(a)
    defer stdout.close()

    stdin buf.Reader = io.stdin(a)
    defer stdin.close()

    out wr.Writer = stdout.writer()

    out.writeLn("Started program. Write 'quit' to exit.")

    while true:
        out.write("Input: ")
        stdout.flush()

        input str = try stdin.readLn(a)
        defer stg.free(a, input)

        out.write("Wrote: ")
        out.writeLn(input)

        if stg.compare(input, "quit"):
            out.writeLn("Bye-bye.")
            break
        ..
    ..
    ret
..