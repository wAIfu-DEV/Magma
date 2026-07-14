mod main

use "../std/heap.mg" heap
use "../std/io.mg" io
use "../std/strconv.mg" strconv
use "../std/strings.mg" strings

readNumber(a alc.Allocator, input buffered.Reader, output buffered.Writer, prompt str) !u64:
    try output.writer().write(prompt)
    try output.flush()

    text := try input.readLn(a)
    defer strings.free(a, text)

    ret try strconv.parseUint(text)
..

use "../std/allocator.mg" alc
use "../std/buffered.mg" buffered

main() !void:
    a := heap.allocator()

    stdout := try io.stdout(a)
    stdin := try io.stdin(a)

    defer stdout.close()
    defer stdin.close()

    out := stdout.writer()

    left := try readNumber(a, stdin, stdout, "First whole number: ")
    right := try readNumber(a, stdin, stdout, "Second whole number: ")

    try out.write("Operator (+, -, *, /): ")
    try stdout.flush()

    op := try stdin.readLn(a)
    defer strings.free(a, op)

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
