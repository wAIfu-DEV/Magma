mod main

Resource(value u64)

destr Resource.close() void:
..

makeResource(value u64) $Resource:
    ret Resource(value=value)
..

consume(value $Resource) void:
    value.close()
..

main() void:
    resource $Resource = makeResource(42)
    consume(move resource)
..
