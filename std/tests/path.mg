mod main
use "../errors.mg" errors
use "../path.mg" path
use "../strings.mg" strings
use "../allocator.mg" allocator
use "../heap.mg" heap
pub main() !void:
    a allocator.Allocator = heap.allocator()
    separator := path.separator()
    if path.isSeparator(separator) == false:
        throw errors.failure("native path separator was not recognized")
    ..
    if separator == 92:
        if path.isAbsolute("C:\\tmp") == false:
            throw errors.failure("absolute Windows path was not recognized")
        ..
    elif path.isAbsolute("/tmp") == false:
        throw errors.failure("absolute Unix path was not recognized")
    ..
    base := try path.base(a, "one/two.txt")
    defer strings.free(a, base)
    if strings.compare(base, "two.txt") == false || strings.toPtr(base)[strings.countBytes(base)] != 0:
        throw errors.failure("path base changed")
    ..
    extension := try path.extension(a, "one/two.txt")
    defer strings.free(a, extension)
    if strings.compare(extension, ".txt") == false || strings.toPtr(extension)[strings.countBytes(extension)] != 0:
        throw errors.failure("path extension changed")
    ..
    noExtension := try path.extension(a, "README")
    defer strings.free(a, noExtension)
    if strings.countBytes(noExtension) != 0 || *strings.toPtr(noExtension) != 0:
        throw errors.failure("empty path extension is not null terminated")
    ..
..
