mod core

use "allocator.mg" alc

# Canonical error predicates. Besides being convenient, these are the only
# predicates used by ownership flow refinement for destructured throwing calls.
error.ok() bool:
    llvm "  %value = load %type.error, ptr %this\n"
    llvm "  %code = extractvalue %type.error %value, 2\n"
    llvm "  %ok = icmp eq i32 %code, 0\n"
    llvm "  ret i1 %ok\n"
..

error.nok() bool:
    llvm "  %value = load %type.error, ptr %this\n"
    llvm "  %code = extractvalue %type.error %value, 2\n"
    llvm "  %nok = icmp ne i32 %code, 0\n"
    llvm "  ret i1 %nok\n"
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
destr str.free(a alc.Allocator) void:
    value str = *this
    data u8* = strData(value)
    if data == none:
        ret
    ..
    a.free(data)
..
