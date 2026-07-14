mod main

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

    try out.write("Text: ")
    try stdout.flush()

    text := try stdin.readLn(a)
    defer strings.free(a, text)

    chars := strings.countBytes(text)
    words u64 = 0
    inside bool = false
    i u64 = 0

    while i < chars:
        if strings.byteAt(text, i) == 32 || strings.byteAt(text, i) == 9:
            inside = false
        elif inside == false:
            words = words + 1
            inside = true
        ..
        i = i + 1
    ..

    try out.write("Characters: ")
    try out.writeUint64(chars)
    try out.write("\nWords: ")
    try out.writeUint64(words)
    try out.writeLn("")
..
