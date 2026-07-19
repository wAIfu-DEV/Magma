mod main
use "../allocator.mg" allocator
use "../heap.mg" heap
use "../io.mg" io
pub main() !void:
    a allocator.Allocator = heap.allocator()
    rawOutput := io.stdoutUnbuffered()
    try rawOutput.writeAll("")
    rawError := io.stderrUnbuffered()
    try rawError.writeAll("")
    rawInput := io.stdinUnbuffered()
    output := try io.stdout(a)
    try output.writer().writeAll("")
    try output.close()
    errorOutput := try io.stderr(a)
    try errorOutput.writer().writeAll("")
    try errorOutput.close()
    input := try io.stdin(a)
    input.close()
..
