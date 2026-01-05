mod slices

# Returns element count of slice.
# @param s input slice
# @returns element count

pub count(s slice) u64:
    llvm "  %l0 = extractvalue %type.slice %s, 1\n"
    llvm "  ret i64 %l0\n"
..

pub fromPtr(p ptr, elemCount u64) slice:
    llvm "  %s0 = insertvalue %type.slice zeroinitializer, ptr %p, 0\n"
    llvm "  %s1 = insertvalue %type.slice %s0, i64 %elemCount, 1\n"
    llvm "  ret %type.slice %s1\n"
..

pub toPtr(s slice) ptr:
    llvm "  %l0 = extractvalue %type.slice %s, 0\n"
    llvm "  ret ptr %l0\n"
..
