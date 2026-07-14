mod slices

use "allocator.mg" alc

# Returns element count of slice.
# O(1).
# @param s input slice
# @returns element count
pub count(s slice) u64:
    llvm "  %l0 = extractvalue %type.slice %s, 1\n"
    llvm "  ret i64 %l0\n"
..

# Creates a slice from a pointer and element count.
# O(1).
# @param p pointer to first element
# @param elemCount number of elements
# @returns slice view
pub fromPtr(p ptr, elemCount u64) slice:
    llvm "  %s0 = insertvalue %type.slice zeroinitializer, ptr %p, 0\n"
    llvm "  %s1 = insertvalue %type.slice %s0, i64 %elemCount, 1\n"
    llvm "  ret %type.slice %s1\n"
..

pub reinterpret[T, R](in T[]) R[]:
    byteSize u64 = count(in) * sizeof T
    newSize u64 = byteSize / sizeof R
    ret fromPtr(toPtr(in), newSize)
..

# Returns the underlying data pointer of a slice.
# O(1).
# @param s input slice
# @returns data pointer
pub toPtr(s slice) ptr:
    llvm "  %l0 = extractvalue %type.slice %s, 0\n"
    llvm "  ret ptr %l0\n"
..

# Frees a allocated slice using the provided allocator.
# Only use if the slice is a owned $T[] slice from a function taking an Allocator
# as parameter.
# Allocator should be the exact same that the slice was allocated with.
# @param a allocator
# @param s allocated slice
pub free(a alc.Allocator, s slice) void:
    p ptr = toPtr(s)
    a.free(p)
..
