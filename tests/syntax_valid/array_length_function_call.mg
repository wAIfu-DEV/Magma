mod main
length() u64: ret 3 ..
pub main() void:
    values := array u8[length()]
    bounded 2 < values.count():
        values[2] = 1
    ..
..
