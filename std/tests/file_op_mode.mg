mod main
use "../errors.mg" errors
use "../file_op_mode.mg" mode
pub main() !void:
    value mode.OpenMode
    readable := value.read()
    writable := readable.write()
    writable.append()
..
