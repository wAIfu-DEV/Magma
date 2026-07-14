mod main

use "../std/fs.mg" fs
use "../std/heap.mg" heap
use "../std/io.mg" io
use "../std/strings.mg" strings

main() !void:
    a := heap.allocator()

    stdout := try io.stdout(a)
    stdin := try io.stdin(a)

    defer stdout.close()
    defer stdin.close()

    out := stdout.writer()

    try out.write("Write a note: ")
    try stdout.flush()

    note := try stdin.readLn(a)
    defer strings.free(a, note)

    try fs.writeFile(a, "notes.txt", note)

    saved := try fs.readFile(a, "notes.txt")
    defer strings.free(a, saved)

    try out.write("Saved note: ")
    try out.writeLn(saved)
..
