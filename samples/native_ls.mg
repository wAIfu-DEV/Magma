mod main
use "std:allocator" as allocator
use "std:errors" as errors
use "std:dialog" as dialog
use "std:fs" as fs
use "std:heap" as heap
use "std:io" as io

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
    defer selectedDir.free()

    try io.printLn(selectedDir)
    directory := try fs.openDir(selectedDir)
    defer directory.close()

    entries := directory.iterator()
    loop entries.hasData():
        entry := try entries.next()
        try io.printLn(entry.name())
    ..
..
