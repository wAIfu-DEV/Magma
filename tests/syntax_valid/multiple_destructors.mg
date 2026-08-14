mod main
Resource(value u64)
destr Resource.close() void: this.value = 0 ..
destr Resource.discard() void: this.value = 0 ..
pub main() void:
    first := Resource(value=0)
    first.close()
    second := Resource(value=0)
    second.discard()
..
