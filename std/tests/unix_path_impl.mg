mod main
use "../unix/path_impl.mg" path_impl
pub main() void:
    path_impl.separator()
    path_impl.isAbsolute("/tmp")
..
