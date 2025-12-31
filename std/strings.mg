mod strings

# Returns size in bytes of string, for UTF8 strings codepoint (UTF8 character) count may be
# different from byte size.
# @param s input string
# @returns size in bytes of string

pub count(s str) u64:
    llvm "  %l0 = extractvalue %type.slice %s, 1\n"
    llvm "  ret u64 %l0\n"
..
