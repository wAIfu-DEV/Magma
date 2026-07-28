mod main
Resource(value u64)
destr Resource.release(code u64) void: this.value = code ..
pub main() void:
    resource Resource
    resource.release(1)
..
