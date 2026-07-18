mod main
Item(value u64)
make() $Item:
    ret Item(value=1)
..
main() void:
    item $Item = make()
..
