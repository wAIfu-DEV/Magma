mod main

use "../std/heap.mg" as heap
use "../std/io.mg" as io
use "../std/random.mg" as random
use "../std/strconv.mg" as strconv
use "../std/strings.mg" as strings
use "../std/time.mg" as time

main() !void:
    a := heap.allocator()

    stdout := try io.stdout()
    stdin := try io.stdin()

    defer stdout.close()
    defer stdin.close()

    out := stdout.writer()

    rng := random.new(time.ticks())
    answer := rng.below(100) + 1

    try out.writeLn("Guess a number from 1 to 100.")
    loop true:
        try out.write("Guess: ")
        try stdout.flush()

        text := try stdin.readLn(a)
        guess := try strconv.parseUint(text)
        text.free()

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
