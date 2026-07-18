mod main
Runner(value u64)
identity(value u64) u64:
    ret value
..
Runner.run(callback (u64) u64) u64:
    ret callback(this.value)
..
main() void:
    runner Runner = Runner(value=3)
    value u64 = runner.run(identity)
..
