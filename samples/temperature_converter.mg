mod main

use "../std/heap.mg" as heap
use "../std/io.mg" as io
use "../std/strconv.mg" as strconv
use "../std/strings.mg" as strings

main() !void:
    a := heap.allocator()

    stdout := try io.stdout()
    stdin := try io.stdin()

    defer stdout.close()
    defer stdin.close()

    out := stdout.writer()

    try out.write("Temperature in Celsius (whole number): ")
    try stdout.flush()

    text := try stdin.readLn(a)
    defer text.free()

    c := try strconv.parseUint(text)
    f := c * 9 / 5 + 32

    try out.write("Fahrenheit: ")
    try out.writeUint64(f)
    try out.writeLn("")
..
