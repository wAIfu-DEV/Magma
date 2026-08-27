mod main

Resource(value u64)
CallbackEntry(context ptr, callback (ptr) void)

Resource.reset() void:
    this.value = 0
..

main() void:
    resource := Resource(value=1)
    entry := CallbackEntry(context=addrof resource, callback=resource.reset)
    entry.callback(entry.context)
..
