mod slices

# Returns element count of slice.
# @param s input slice
# @returns element count

pub count(s slice) u64:
    llvm "  %l0 = extractvalue %type.slice %s, 1\n"
    llvm "  ret i64 %l0\n"
..
