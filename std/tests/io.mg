mod main
use "std:allocator" allocator
use "std:heap" heap
use "std:io" io
pub main() !void:
    a allocator.Allocator = heap.allocator()
    rawOutput := io.stdoutUnbuffered()
    try rawOutput.writeAll("")
    rawError := io.stderrUnbuffered()
    try rawError.writeAll("")
    rawInput := io.stdinUnbuffered()
    output := try io.stdout()
    try output.writer().writeAll("")
    try output.close()
    errorOutput := try io.stderr()
    try errorOutput.writer().writeAll("")
    try errorOutput.close()
    input := try io.stdin()
    input.close()
..
