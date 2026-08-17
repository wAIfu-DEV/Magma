mod main
use "std:allocator" allocator
use "std:errors" errors
use "std:fs" fs
use "std:heap" heap
use "std:strings" strings

found bool

visit(path str, isDirectory bool) !void:
    if strings.compare(path, "std\\fs.mg") || strings.compare(path, "std/fs.mg"):
        found = true
    ..
..

pub main() !void:
    a allocator.Allocator = heap.allocator()
    try fs.writeFile("std_checked_test_fs.tmp", "checked fs")
    contents := try fs.readFile("std_checked_test_fs.tmp")
    defer contents.free(a)
    if strings.compare(contents, "checked fs") == false:
        throw errors.failure("filesystem behavior changed")
    ..
    try fs.walk("std", visit)
    if found == false:
        throw errors.failure("directory walk did not find std/fs.mg")
    ..
    try fs.removeFile("std_checked_test_fs.tmp")
    missing str, missingErr error = fs.readFile("std_checked_test_fs.tmp")
    if missingErr.ok():
        missing.free(a)
        throw errors.failure("removed file can still be opened")
    ..

    root := "std_phase5_fs_test"
    nested := "std_phase5_fs_test/a/b"
    try fs.makeDirs(nested)
    try fs.writeFile("std_phase5_fs_test/a/b/source.bin", "phase five")
    info := try fs.metadata("std_phase5_fs_test/a/b/source.bin")
    if info.kind().isFile() == false || info.size() != 10:
        throw errors.failure("file metadata changed")
    ..
    directory := try fs.openDir(nested)
    entryFound bool = false
    entries := directory.iterator()
    loop entries.hasData():
        entry := try entries.next()
        if strings.compare(entry.name(), "source.bin"):
            entryFound = true
        ..
    ..
    try directory.close()
    if entryFound == false:
        throw errors.failure("directory iteration missed a file")
    ..
    try fs.copyFile("std_phase5_fs_test/a/b/source.bin", "std_phase5_fs_test/copy.bin")
    try fs.rename("std_phase5_fs_test/copy.bin", "std_phase5_fs_test/renamed.bin")
    try fs.writeFile("std_phase5_fs_test/replacement.bin", "replacement")
    try fs.replace("std_phase5_fs_test/replacement.bin", "std_phase5_fs_test/renamed.bin")
    replacement := try fs.readFile("std_phase5_fs_test/renamed.bin")
    defer replacement.free(a)
    if strings.compare(replacement, "replacement") == false:
        throw errors.failure("atomic replacement changed")
    ..
    current := try fs.currentDir()
    defer current.free(a)
    temporary := try fs.temporaryDir()
    defer temporary.free(a)
    canonical := try fs.canonicalize(root)
    defer canonical.free(a)
    if current.countBytes() == 0 || temporary.countBytes() == 0 || canonical.countBytes() == 0:
        throw errors.failure("system directory query returned an empty path")
    ..
    try fs.removeTree(root)
..
