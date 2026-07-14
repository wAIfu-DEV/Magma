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

    tasks str[32]
    count u64 = 0
    running bool = true

    while running:
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

            while i < count:
                try out.writeUint64(i + 1)
                try out.write(". ")
                try out.writeLn(tasks[i])
                i = i + 1
            ..
        elif strings.compare(choice, "3"):
            running = false
        ..

        strings.free(a, choice)
    ..

    cleanupIndex u64 = 0

    while cleanupIndex < count:
        strings.free(a, tasks[cleanupIndex])
        cleanupIndex = cleanupIndex + 1
    ..
..
