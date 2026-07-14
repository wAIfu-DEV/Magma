mod main
use "../errors.mg" errors
use "../cast.mg" cast
use "../strings.mg" strings
use "../writer.mg" writer
sink(impl ptr, bytes str) !u64:
    ret strings.countBytes(bytes)
..
pub main() !void:
    output := writer.new(cast.utop(0), sink)
    count := try output.writeLn("abc")
    if count != 4:
        throw errors.failure("writer behavior changed")
    ..
..
