mod main
Resource(value u64)
destr Resource.close() void: this.value = 0 ..
destr Resource.discard() void: this.value = 0 ..
pub main() void:
    first Resource
    first.close()
    second Resource
    second.discard()
..
