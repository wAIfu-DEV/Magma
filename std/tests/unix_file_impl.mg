mod main
use "../unix/file_impl.mg" file_impl
pub main() !void:
    output := file_impl.stdout()
    try output.writeAll("")
..
