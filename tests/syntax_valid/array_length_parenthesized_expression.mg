mod main
pub main() void:
    count u64 = 2
    values := array u8[(count + 1) * 2]
    bounded 5 < values.count():
        values[5] = 1
    ..
..
