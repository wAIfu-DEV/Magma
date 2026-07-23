mod main
use "std:errors" errors
use "std:file_op_mode" mode
pub main() !void:
    value mode.OpenMode
    readable := value.read()
    writable := readable.write()
    writable.append()
..
