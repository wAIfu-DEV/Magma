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
const streamVTable := duplex.Vtable(
    fn_write = write,
    fn_read =  read,
)
pub main() !void:
    stream := duplex.new(none, addrof streamVTable)
    count := try stream.writer().write("ok")
    if count != 2:
        throw errors.failure("duplex behavior changed")
    ..
..
