mod main
@platform("definitely-not-a-real-platform")
ext skipped_symbol skipped_symbol() void
raw(value u64) u64:
    llvm "  ret i64 %value\n"
..
main() void:
    value u64 = raw(1)
..
