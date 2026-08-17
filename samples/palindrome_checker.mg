mod main

use "../std/heap.mg" heap
use "../std/io.mg" io
use "../std/strings.mg" strings

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
    defer word.free(a)

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
