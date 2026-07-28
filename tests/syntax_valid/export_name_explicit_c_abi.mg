mod main
@export_name("magma_test_export", "C")
exported(value i32) i32: ret value ..
pub main() void:
    value := exported(1)
..
