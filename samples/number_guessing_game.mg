mod main

use "../std/heap.mg" heap
use "../std/io.mg" io
use "../std/random.mg" random
use "../std/strconv.mg" strconv
use "../std/strings.mg" strings
use "../std/time.mg" time

main() !void:
    a := heap.allocator()

    stdout := try io.stdout(a)
    stdin := try io.stdin(a)

    defer stdout.close()
    defer stdin.close()

    out := stdout.writer()

    rng := random.new(time.ticks())
    answer := rng.bounded(100) + 1

    try out.writeLn("Guess a number from 1 to 100.")
    while true:
        try out.write("Guess: ")
        try stdout.flush()

        text := try stdin.readLn(a)
        guess := try strconv.parseUint(text)
        strings.free(a, text)

        if guess < answer:
            try out.writeLn("Too low.")
        elif guess > answer:
            try out.writeLn("Too high.")
        else:
            try out.writeLn("Correct!")
            break
        ..
    ..
..
