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

    tasks := array str[32]
    count u64 = 0
    running bool = true

    loop running:
        try out.writeLn("\n1. Add task  2. List tasks  3. Quit")
        try out.write("Choice: ")
        try stdout.flush()

        choice := try stdin.readLn(a)

        if strings.compare(choice, "1") && count < 32:
            try out.write("Task: ")
            try stdout.flush()
            tasks[count] = try stdin.readLn(a)
            count = count + 1

        elif strings.compare(choice, "2"):
            i u64 = 0

            loop i < count:
                try out.writeUint64(i + 1)
                try out.write(". ")
                try out.writeLn(tasks[i])
                i = i + 1
            ..
        elif strings.compare(choice, "3"):
            running = false
        ..

        choice.free(a)
    ..

    cleanupIndex u64 = 0

    # SAFETY: entries below count were initialized exactly once and this final
    # loop uniquely destroys each owned task string.
    unsafe:
    loop cleanupIndex < count:
        tasks[cleanupIndex].free(a)
        cleanupIndex = cleanupIndex + 1
    ..
    ..
..
