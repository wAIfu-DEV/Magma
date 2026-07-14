mod main

use "../std/heap.mg" heap
use "../std/io.mg" io
use "../std/strconv.mg" strconv
use "../std/strings.mg" strings

main() !void:
    a := heap.allocator()

    stdout := try io.stdout(a)
    stdin := try io.stdin(a)

    defer stdout.close()
    defer stdin.close()

    out := stdout.writer()

    try out.write("Temperature in Celsius (whole number): ")
    try stdout.flush()

    text := try stdin.readLn(a)
    defer strings.free(a, text)

    c := try strconv.parseUint(text)
    f := c * 9 / 5 + 32

    try out.write("Fahrenheit: ")
    try out.writeUint64(f)
    try out.writeLn("")
..
