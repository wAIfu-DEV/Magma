mod main

use "../std/heap.mg" heap
use "../std/io.mg" io
use "../std/strconv.mg" strconv
use "../std/strings.mg" strings

won(board u8[], mark u8) bool:
    if board[0] == mark && board[1] == mark && board[2] == mark:
        ret true
    elif board[3] == mark && board[4] == mark && board[5] == mark:
        ret true
    elif board[6] == mark && board[7] == mark && board[8] == mark:
        ret true
    elif board[0] == mark && board[3] == mark && board[6] == mark:
        ret true
    elif board[1] == mark && board[4] == mark && board[7] == mark:
        ret true
    elif board[2] == mark && board[5] == mark && board[8] == mark:
        ret true
    elif board[0] == mark && board[4] == mark && board[8] == mark:
        ret true
    elif board[2] == mark && board[4] == mark && board[6] == mark:
        ret true
    ..
    ret false
..

printBoard(out writer.Writer, board u8[]) !void:
    i u64 = 0

    while i < 9:
        if board[i] == 0:
            try out.writeUint64(i + 1)
        else:
            cellByte u8 = board[i]
            cell := strings.fromPtrNoCopy(addrof cellByte, 1)
            try out.write(cell)
        ..
        if i % 3 == 2:
            try out.writeLn("")
        else:
            try out.write(" | ")
        ..

        i = i + 1
    ..
..

use "../std/writer.mg" writer

main() !void:
    a := heap.allocator()

    stdout := try io.stdout(a)
    stdin := try io.stdin(a)

    defer stdout.close()
    defer stdin.close()

    out := stdout.writer()

    board := array u8[9]
    turn u64 = 0

    while turn < 9:
        try printBoard(out, board)

        mark u8 = 88
        if turn % 2 == 1:
            mark = 79
        ..

        try out.write("Player ")
        try out.write(strings.fromPtrNoCopy(addrof mark, 1))
        try out.write(", choose 1-9: ")
        try stdout.flush()

        text := try stdin.readLn(a)
        position := try strconv.parseUint(text)
        strings.free(a, text)

        if position >= 1 && position <= 9 && board[position - 1] == 0:
            board[position - 1] = mark
            turn = turn + 1

            if won(board, mark):
                try printBoard(out, board)
                try out.writeLn("Winner!")
                ret
            ..
        else:
            try out.writeLn("That square is unavailable.")
        ..
    ..

    try printBoard(out, board)
    try out.writeLn("Draw.")
..
