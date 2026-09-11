mod main

use "../std/heap.mg" as heap
use "../std/io.mg" as io
use "../std/strings.mg" as strings

main() !void:
    a := heap.allocator()

    stdout := try io.stdout()
    stdin := try io.stdin()

    defer stdout.close()
    defer stdin.close()

    out := stdout.writer()

    try out.write("Word: ")
    try stdout.flush()

    word := try stdin.readLn(a)
    defer word.free()

    count := word.countBytes()
    palindrome bool = true
    i u64 = 0

    loop i < count / 2:
        if strings.byteAt(word, i) != strings.byteAt(word, count - i - 1):
            palindrome = false
            break
        ..
        i = i + 1
    ..

    if palindrome:
        try out.writeLn("It is a palindrome.")
    else:
        try out.writeLn("It is not a palindrome.")
    ..
..
