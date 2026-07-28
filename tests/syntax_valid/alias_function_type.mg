mod main
alias Callback = (u64) u64
increment(value u64) u64: ret value + 1 ..
pub main() void:
    callback Callback = increment
    value := callback(2)
..
