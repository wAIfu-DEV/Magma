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

    names str[32]
    phones str[32]
    count u64 = 0
    running bool = true

    while running:
        try out.writeLn("\n1. Add contact  2. Find contact  3. Quit")
        try out.write("Choice: ")
        try stdout.flush()

        choice := try stdin.readLn(a)

        if strings.compare(choice, "1") && count < 32:
            try out.write("Name: ")
            try stdout.flush()

            names[count] = try stdin.readLn(a)

            try out.write("Phone: ")
            try stdout.flush()
            
            phones[count] = try stdin.readLn(a)
            count = count + 1

        elif strings.compare(choice, "2"):
            try out.write("Name to find: ")
            try stdout.flush()
            query := try stdin.readLn(a)
            i u64 = 0
            found bool = false

            while i < count:
                if strings.compare(names[i], query):
                    try out.write("Phone: ")
                    try out.writeLn(phones[i])
                    found = true
                ..
                i = i + 1
            ..

            if found == false:
                try out.writeLn("Contact not found.")
            ..
            strings.free(a, query)
        elif strings.compare(choice, "3"):
            running = false
        ..

        strings.free(a, choice)
    ..

    cleanupIndex u64 = 0

    while cleanupIndex < count:
        strings.free(a, names[cleanupIndex])
        strings.free(a, phones[cleanupIndex])
        cleanupIndex = cleanupIndex + 1
    ..
..
