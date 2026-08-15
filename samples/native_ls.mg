mod main
use "std:allocator" allocator
use "std:errors" errors
use "std:dialog" dialog
use "std:fs" fs
use "std:heap" heap
use "std:io" io

pub main() !void:
    a := heap.allocator()

    configuration := dialog.defaultOptions()
    selectedDir, dialogError := dialog.openDir(a, configuration)
    if dialogError.nok():
        if errors.is(dialogError, errors.cancelled("")):
            try io.printLn("cancelled")
            ret
        ..
        throw dialogError
    ..
    defer selectedDir.free(a)

    try io.printLn(selectedDir)
    directory := try fs.openDir(a, selectedDir)
    defer directory.close()

    entries := directory.iterator()
    loop entries.hasData():
        entry := try entries.next()
        try io.printLn(entry.name())
    ..
..
