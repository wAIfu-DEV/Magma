mod file_op_mode
# Composable flags controlling how files are opened.

# File open mode flags.
# @complexity O(1).
pub OpenMode(
    r bool
    w bool
    a bool
    c bool
    t bool
)

# Returns a copy of the mode with read access enabled.
# @complexity O(1)
# @example
#   openMode := file.mode().read()
OpenMode.read() OpenMode:
    op OpenMode = *this
    op.r = true
    ret op
..

# Returns a copy of the mode with write access enabled.
# @complexity O(1)
OpenMode.write() OpenMode:
    op OpenMode = *this
    op.w = true
    ret op
..

# Returns a copy configured to append writes at the end of the file.
# This also enables write access.
# @complexity O(1)
OpenMode.append() OpenMode:
    op OpenMode = *this
    op.a = true
    op.w = true
    ret op
..

# Creates the file when it does not exist. Existing contents are preserved.
# @complexity O(1)
OpenMode.create() OpenMode:
    op OpenMode = *this
    op.c = true
    ret op
..

# Truncates an existing file to zero bytes when opened. This also enables
# writing, but does not imply creation.
# @complexity O(1)
# @warning Opening with this mode destroys existing file contents.
# @example
#   openMode := file.mode().write().create().truncate()
OpenMode.truncate() OpenMode:
    op OpenMode = *this
    op.t = true
    op.w = true
    ret op
..
