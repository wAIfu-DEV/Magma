mod main
use "std:duplex" duplex
use "std:cast" cast
use "std:errors" errors
use "std:strings" strings
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
    buffer := array u8[1]
    if try stream.reader().readToBuff(buffer, 1) != 0:
        throw errors.failure("duplex reader changed")
    ..
..
