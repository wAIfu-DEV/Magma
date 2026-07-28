mod main
Resource(value u64)
destr Resource.release() !void: this.value = 0 ..
pub main() !void:
    resource Resource
    try resource.release()
..
