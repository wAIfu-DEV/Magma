mod main

use "../std/heap.mg" as heap
use "../std/io.mg" as io
use "../std/strconv.mg" as strconv
use "../std/strings.mg" as strings

readNumber(a alc.Allocator, input buffered.Reader, output buffered.Writer, prompt str) !u64:
    try output.writer().write(prompt)
    try output.flush()

    text := try input.readLn(a)
    defer text.free()

    ret try strconv.parseUint(text)
..

use "../std/allocator.mg" as alc
use "../std/buffered.mg" as buffered

main() !void:
    a := heap.allocator()

    stdout := try io.stdout()
    stdin := try io.stdin()

    defer stdout.close()
    defer stdin.close()

    out := stdout.writer()

    left := try readNumber(a, stdin, stdout, "First whole number: ")
    right := try readNumber(a, stdin, stdout, "Second whole number: ")

    try out.write("Operator (+, -, *, /): ")
    try stdout.flush()

    op := try stdin.readLn(a)
    defer op.free()

    result u64
    if strings.compare(op, "+"):
        result = left + right
    elif strings.compare(op, "-"):
        result = left - right
    elif strings.compare(op, "*"):
        result = left * right
    else:
        result = left / right
    ..

    try out.write("Result: ")
    try out.writeUint64(result)
    try out.writeLn("")
..
