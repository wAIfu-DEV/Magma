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
    if try output.write("ab") != 2 || try output.writeAll("abc") != 3:
        throw errors.failure("writer write behavior changed")
    ..
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
    if try output.writeBool(true) != 4 || try output.writeBool(false) != 5:
        throw errors.failure("writer bool behavior changed")
    ..
    if try output.writeInt64(-42) != 3 || try output.writeUint64(42) != 2:
        throw errors.failure("writer integer behavior changed")
    ..
..
