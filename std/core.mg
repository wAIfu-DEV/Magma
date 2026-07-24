mod core
# Intrinsic types and methods imported implicitly by every Magma program.

use "std:allocator" alc

# Returns the number of elements in a slice.
# @complexity O(1)
# @example
#   length := values.count()
slice.count() u64:
    llvm "  %value = load %type.slice, ptr %this\n"
    llvm "  %cnt = extractvalue %type.slice %value, 1\n"
    llvm "  ret i64 %cnt\n"
..

# Canonical error predicates. Besides being convenient, these are the only
# predicates used by ownership flow refinement for destructured throwing calls.
# @complexity O(1)
# @example
#   if resultError.ok():
error.ok() bool:
    llvm "  %value = load %type.error, ptr %this\n"
    llvm "  %code = extractvalue %type.error %value, 2\n"
    llvm "  %ok = icmp eq i32 %code, 0\n"
    llvm "  ret i1 %ok\n"
..

# Reports whether an error represents failure.
# @complexity O(1)
# @example
#   if resultError.nok():
error.nok() bool:
    llvm "  %value = load %type.error, ptr %this\n"
    llvm "  %code = extractvalue %type.error %value, 2\n"
    llvm "  %nok = icmp ne i32 %code, 0\n"
    llvm "  ret i1 %nok\n"
..

# Returns the error code of an error.
# A code of 0 indicates a successful operation.
# @complexity O(1).
# @returns error code
# @example
#   category := failure.code()
error.code() u32:
    llvm "  %value = load %type.error, ptr %this\n"
	llvm "  %e0 = extractvalue %type.error %value, 2\n"
    llvm "  ret i32 %e0\n"
..

# Returns the message from an error.
# @complexity O(1).
# @returns error message
# @example
#   detail := failure.message()
error.message() str:
    llvm "  %value = load %type.error, ptr %this\n"
	llvm "  %ep = extractvalue %type.error %value, 0\n"
	llvm "  %el = extractvalue %type.error %value, 3\n"
	llvm "  %el64 = zext i32 %el to i64\n"
	llvm "  %s0 = insertvalue %type.str zeroinitializer, ptr %ep, 0\n"
	llvm "  %s1 = insertvalue %type.str %s0, i64 %el64, 1\n"
	llvm "  ret %type.str %s1\n"
..

# Methods declared on primitive types form Magma's implicit core method set.
# The compiler passes a pointer to the receiver as `this`, just as it does for
# struct member functions.
strData(value str) u8*:
    llvm "  %data = extractvalue %type.str %value, 0\n"
    llvm "  ret ptr %data\n"
..

# Releases the backing allocation of an owned string. Borrowed strings and
# literals are not ownership obligations and must not be passed to this method.
# @complexity O(1), excluding allocator cost
# @param a allocator that created the owned string
# @warning Passing a borrowed string or the wrong allocator is invalid.
# @example
#   owned.free(a)
destr str.free(a alc.Allocator) void:
    value str = *this
    data u8* = strData(value)
    if data == none:
        ret
    ..
    a.free(data)
..
