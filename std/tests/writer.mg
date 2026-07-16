mod main
use "../errors.mg" errors
use "../cast.mg" cast
use "../strings.mg" strings
use "../writer.mg" writer
sink(impl ptr, bytes str) !u64:
    ret strings.countBytes(bytes)
..
pub main() !void:
    output := writer.new(none, sink)
    count := try output.writeLn("abc")
    if count != 4:
        throw errors.failure("writer behavior changed")
    ..
    floatCount := try output.writeFloat64(1.5, 1)
    if floatCount != 3:
        throw errors.failure("writer float behavior changed")
    ..
    negativeCount := try output.writeFloat64(-2.0, 0)
    if negativeCount != 2:
        throw errors.failure("writer negative float behavior changed")
    ..
..
