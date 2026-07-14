mod main
use "../duplex.mg" duplex
use "../cast.mg" cast
use "../errors.mg" errors
use "../strings.mg" strings
write(impl ptr, bytes str) !u64:
    ret strings.countBytes(bytes)
..
read(impl ptr, bytes u8[], count u64) !u64:
    ret 0
..
pub main() !void:
    stream := duplex.new(cast.utop(0), write, read)
    count := try stream.writer().write("ok")
    if count != 2:
        throw errors.failure("duplex behavior changed")
    ..
..
