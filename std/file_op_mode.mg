mod file_op_mode

# File open mode flags.
# O(1).
OpenMode(
    r bool
    w bool
    a bool
)

OpenMode.read() OpenMode:
    op OpenMode = *this
    op.r = true
    ret op
..

OpenMode.write() OpenMode:
    op OpenMode = *this
    op.w = true
    ret op
..

OpenMode.append() OpenMode:
    op OpenMode = *this
    op.a = true
    op.w = true
    ret op
..
