mod slices
# Low-level slice construction, allocation, reinterpretation, and release.

use "std:allocator" alc

# Returns element count of slice.
# @complexity O(1).
# @param s input slice
# @returns element count
# @example
#   length := slices.count(values)
pub count(s slice) u64:
    llvm "  %l0 = extractvalue %type.slice %s, 1\n"
    llvm "  ret i64 %l0\n"
..

# Creates a slice from a pointer and element count.
# @complexity O(1).
# @param p pointer to first element
# @param elemCount number of elements
# @returns slice view
# @safety p must reference at least elemCount valid elements.
# @example
#   view := slices.fromPtr(pointer, 16)
pub fromPtr(p ptr, elemCount u64) slice:
    llvm "  %s0 = insertvalue %type.slice zeroinitializer, ptr %p, 0\n"
    llvm "  %s1 = insertvalue %type.slice %s0, i64 %elemCount, 1\n"
    llvm "  ret %type.slice %s1\n"
..

# Reinterprets a slice's backing memory as elements of another type.
# The result length is rounded down if the byte size is not divisible by sizeof R.
# @complexity O(1)
# @param in source slice
# @returns non-owning view over the same backing memory
# @safety The backing memory must satisfy R's alignment and representation requirements.
# @example
#   words := slices.reinterpret[u8, u32](bytes)
pub reinterpret[T, R](in T[]) R[]:
    byteSize u64 = count(in) * sizeof T
    newSize u64 = byteSize / sizeof R
    ret fromPtr(toPtr(in), newSize)
..

# Returns the underlying data pointer of a slice.
# @complexity O(1).
# @param s input slice
# @returns data pointer
# @example
#   pointer := slices.toPtr(values)
pub toPtr(s slice) ptr:
    llvm "  %l0 = extractvalue %type.slice %s, 0\n"
    llvm "  ret ptr %l0\n"
..

# Allocates an owned, uninitialized slice of T values.
# @complexity O(1), excluding allocator cost
# @param a allocator used for the backing memory
# @param elemCount number of elements to allocate
# @returns owned slice with elemCount elements
# @throws outOfMemory when allocation fails
# @ownership Release the result with free using the same allocator.
# @example
#   values := try slices.alloc[u64](a, 16)
#   slices.free(a, values)
pub alloc[T](a alc.Allocator, elemCount u64) !$T[]:
    p T* = try a.allocT[T](elemCount)
    ret fromPtr(p, elemCount)
..

# Releases the backing memory of an owned slice.
# @complexity O(1), excluding allocator cost
# @param a allocator that originally allocated the slice
# @param s owned slice to release
# @warning Passing a borrowed slice or a different allocator is invalid.
# @ownership Consumes the slice and releases its allocation.
# @example
#   slices.free(a, values)
pub free(a alc.Allocator, s slice) void:
    p ptr = toPtr(s)
    a.free(p)
..
