mod dialog_impl_win
# Modern Windows file dialogs implemented directly with IFileDialog.
# Derived from Native File Dialog 1.1.6; see licenses/NATIVE_FILE_DIALOG.txt.

use "std:allocator" allocator
use "std:builder" builder
use "std:c" c
use "std:cast" cast
use "std:errors" errors
use "std:slices" slices
use "std:strings" strings
use "std:utf8" utf8

link "ole32"
link "shell32"

llvm "@magma.filedialog.clsid.open = private constant { i32, i16, i16, [8 x i8] } { i32 -602121572, i16 -6006, i16 19934, [8 x i8] [i8 -91, i8 -95, i8 96, i8 -8, i8 42, i8 32, i8 -82, i8 -9] }\n"
llvm "@magma.filedialog.clsid.save = private constant { i32, i16, i16, [8 x i8] } { i32 -1061887245, i16 -17887, i16 18291, [8 x i8] [i8 -115, i8 -70, i8 51, i8 94, i8 -55, i8 70, i8 -21, i8 -117] }\n"
llvm "@magma.filedialog.iid.open = private constant { i32, i16, i16, [8 x i8] } { i32 -713264504, i16 -11091, i16 18280, [8 x i8] [i8 -66, i8 2, i8 -99, i8 -106, i8 -107, i8 50, i8 -39, i8 96] }\n"
llvm "@magma.filedialog.iid.save = private constant { i32, i16, i16, [8 x i8] } { i32 -2068001501, i16 24542, i16 19675, [8 x i8] [i8 -82, i8 -92, i8 -81, i8 100, i8 -72, i8 61, i8 120, i8 -85] }\n"
llvm "@magma.filedialog.iid.shellitem = private constant { i32, i16, i16, [8 x i8] } { i32 1132621086, i16 -6376, i16 17134, [8 x i8] [i8 -68, i8 85, i8 -95, i8 -30, i8 97, i8 -61, i8 123, i8 -2] }\n"

ext ext_CoInitializeEx CoInitializeEx(reserved ptr, flags u32) i32
ext ext_CoUninitialize CoUninitialize() void
ext ext_CoCreateInstance CoCreateInstance(classId ptr, outer ptr, context u32, interfaceId ptr, result ptr*) i32
ext ext_CoTaskMemFree CoTaskMemFree(value ptr) void
ext ext_SHCreateItemFromParsingName SHCreateItemFromParsingName(path u16*, context ptr, interfaceId ptr, result ptr*) i32

guidOpenClass() ptr:
    llvm "ret ptr @magma.filedialog.clsid.open\n"
..
guidSaveClass() ptr:
    llvm "ret ptr @magma.filedialog.clsid.save\n"
..
guidOpenInterface() ptr:
    llvm "ret ptr @magma.filedialog.iid.open\n"
..
guidSaveInterface() ptr:
    llvm "ret ptr @magma.filedialog.iid.save\n"
..
guidShellItem() ptr:
    llvm "ret ptr @magma.filedialog.iid.shellitem\n"
..

InputFilter(
    name str
    extensions str
)

FilterSpec(
    name u16*
    pattern u16*
)

FileDialogVtable(
    queryInterface (ptr, ptr, ptr*) i32
    addRef (ptr) u32
    release (ptr) u32
    show (ptr, ptr) i32
    setFileTypes (ptr, u32, FilterSpec*) i32
    setFileTypeIndex ptr
    getFileTypeIndex ptr
    advise ptr
    unadvise ptr
    setOptions (ptr, u32) i32
    getOptions (ptr, u32*) i32
    setDefaultFolder (ptr, ptr) i32
    setFolder ptr
    getFolder ptr
    getCurrentSelection ptr
    setFileName (ptr, u16*) i32
    getFileName ptr
    setTitle (ptr, u16*) i32
    setOkButtonLabel ptr
    setFileNameLabel ptr
    getResult (ptr, ptr*) i32
    addPlace ptr
    setDefaultExtension ptr
    close ptr
    setClientGuid ptr
    clearClientData ptr
    setFilter ptr
)

ShellItemVtable(
    queryInterface ptr
    addRef ptr
    release (ptr) u32
    bindToHandler ptr
    getParent ptr
    getDisplayName (ptr, u32, u16**) i32
    getAttributes ptr
    compare ptr
)

succeeded(result i32) bool:
    ret result >= 0
..

cancelled(result i32) bool:
    ret result == cast.u32toi32(0x800704C7)
..

dialogTable(dialog ptr) FileDialogVtable*:
    address ptr* = dialog
    ret cast.reinterpret[FileDialogVtable](*address)
..

shellTable(item ptr) ShellItemVtable*:
    address ptr* = item
    ret cast.reinterpret[ShellItemVtable](*address)
..

releaseDialog(dialog ptr) void:
    if dialog != none:
        table := dialogTable(dialog)
        table.release(dialog)
    ..
..

releaseShell(item ptr) void:
    if item != none:
        table := shellTable(item)
        table.release(item)
    ..
..

finishCom(result i32) void:
    if result == 0 || result == 1:
        ext_CoUninitialize()
    ..
..

startCom() !i32:
    result := ext_CoInitializeEx(none, 2)
    # RPC_E_CHANGED_MODE means COM was already initialized in another model.
    if succeeded(result) == false && result != cast.u32toi32(0x80010106):
        throw errors.native(cast.u64to32(cast.itou(cast.i32to64(result))), "CoInitializeEx failed")
    ..
    ret result
..

createDialog(save bool) !ptr:
    dialog ptr = none
    result i32 = 0
    if save:
        result = ext_CoCreateInstance(guidSaveClass(), none, 23, guidSaveInterface(), addrof dialog)
    else:
        result = ext_CoCreateInstance(guidOpenClass(), none, 23, guidOpenInterface(), addrof dialog)
    ..
    if succeeded(result) == false:
        throw errors.native(cast.u64to32(cast.itou(cast.i32to64(result))), "CoCreateInstance for IFileDialog failed")
    ..
    ret dialog
..

extensionPattern(a allocator.Allocator, extensions str) !$str:
    out := try builder.new(a)
    defer out.free()
    start u64 = 0
    i u64 = 0
    while i <= extensions.countBytes():
        if i == extensions.countBytes() || strings.byteAt(extensions, i) == 44:
            if i > start:
                if out.isEmpty() == false:
                    try out.appendBorrowed(";")
                ..
                try out.appendBorrowed("*.")
                try out.appendBorrowed(strings.fromPtrNoCopy(cast.utop(cast.ptou(strings.toPtr(extensions)) + start), i - start))
            ..
            start = i + 1
        ..
        i = i + 1
    ..
    if out.isEmpty():
        try out.appendBorrowed("*.*")
    ..
    ret try out.build()
..

setFilters(a allocator.Allocator, dialog ptr, rawFilters ptr, count u64) !void:
    if count == 0:
        ret
    ..
    filters InputFilter* = rawFilters
    specs := try a.allocT[FilterSpec](count)
    names := try a.allocT[u16*](count)
    patterns := try a.allocT[u16*](count)
    built u64 = 0
    while built < count:
        names[built] = none
        patterns[built] = none
        built = built + 1
    ..
    built = 0
    while built < count:
        nameUnits := try utf8.utf8To16NT(a, filters[built].name)
        pattern := try extensionPattern(a, filters[built].extensions)
        patternUnits := try utf8.utf8To16NT(a, pattern)
        pattern.free(a)
        names[built] = slices.toPtr(nameUnits)
        patterns[built] = slices.toPtr(patternUnits)
        specs[built] = FilterSpec(name=names[built], pattern=patterns[built])
        built = built + 1
    ..
    table := dialogTable(dialog)
    result := table.setFileTypes(dialog, cast.u64to32(count), specs)
    i u64 = 0
    while i < count:
        a.free(names[i])
        a.free(patterns[i])
        i = i + 1
    ..
    a.free(names)
    a.free(patterns)
    a.free(specs)
    if succeeded(result) == false:
        throw errors.native(cast.u64to32(cast.itou(cast.i32to64(result))), "IFileDialog.SetFileTypes failed")
    ..
..

setDefaultPath(a allocator.Allocator, dialog ptr, path str) !void:
    if path.countBytes() == 0:
        ret
    ..
    wide := try utf8.utf8To16NT(a, path)
    defer slices.free(a, wide)
    item ptr = none
    result := ext_SHCreateItemFromParsingName(slices.toPtr(wide), none, guidShellItem(), addrof item)
    if succeeded(result) == false:
        throw errors.native(cast.u64to32(cast.itou(cast.i32to64(result))), "default dialog path is invalid")
    ..
    defer releaseShell(item)
    table := dialogTable(dialog)
    result = table.setDefaultFolder(dialog, item)
    if succeeded(result) == false:
        throw errors.native(cast.u64to32(cast.itou(cast.i32to64(result))), "IFileDialog.SetDefaultFolder failed")
    ..
..

configureText(a allocator.Allocator, dialog ptr, title str, defaultName str) !void:
    table := dialogTable(dialog)
    if title.countBytes() > 0:
        wideTitle := try utf8.utf8To16NT(a, title)
        result := table.setTitle(dialog, slices.toPtr(wideTitle))
        slices.free(a, wideTitle)
        if succeeded(result) == false:
            throw errors.native(cast.u64to32(cast.itou(cast.i32to64(result))), "IFileDialog.SetTitle failed")
        ..
    ..
    if defaultName.countBytes() > 0:
        wideName := try utf8.utf8To16NT(a, defaultName)
        result := table.setFileName(dialog, slices.toPtr(wideName))
        slices.free(a, wideName)
        if succeeded(result) == false:
            throw errors.native(cast.u64to32(cast.itou(cast.i32to64(result))), "IFileDialog.SetFileName failed")
        ..
    ..
..

selectionFromDialog(a allocator.Allocator, dialog ptr) !$str:
    table := dialogTable(dialog)
    item ptr = none
    result := table.getResult(dialog, addrof item)
    if succeeded(result) == false:
        throw errors.native(cast.u64to32(cast.itou(cast.i32to64(result))), "IFileDialog.GetResult failed")
    ..
    defer releaseShell(item)
    shell := shellTable(item)
    wide u16* = none
    result = shell.getDisplayName(item, 0x80058000, addrof wide)
    if succeeded(result) == false:
        throw errors.native(cast.u64to32(cast.itou(cast.i32to64(result))), "IShellItem.GetDisplayName failed")
    ..
    defer ext_CoTaskMemFree(wide)
    count u64 = 0
    while wide[count] != 0:
        count = count + 1
    ..
    path := try utf8.utf16to8(a, slices.fromPtr(wide, count))
    ret path
..

show(a allocator.Allocator, save bool, directory bool, rawFilters ptr, filterCount u64, defaultPath str, defaultName str, title str, parent ptr) !$str:
    comResult := try startCom()
    defer finishCom(comResult)
    dialog := try createDialog(save)
    defer releaseDialog(dialog)
    try setFilters(a, dialog, rawFilters, filterCount)
    try setDefaultPath(a, dialog, defaultPath)
    try configureText(a, dialog, title, defaultName)
    if directory:
        table := dialogTable(dialog)
        options u32 = 0
        result := table.getOptions(dialog, addrof options)
        if succeeded(result) == false:
            throw errors.native(cast.u64to32(cast.itou(cast.i32to64(result))), "IFileDialog.GetOptions failed")
        ..
        result = table.setOptions(dialog, options | 0x20)
        if succeeded(result) == false:
            throw errors.native(cast.u64to32(cast.itou(cast.i32to64(result))), "IFileDialog.SetOptions failed")
        ..
    ..
    table := dialogTable(dialog)
    result := table.show(dialog, parent)
    if cancelled(result):
        throw errors.cancelled("file dialog cancelled")
    elif succeeded(result) == false:
        throw errors.native(cast.u64to32(cast.itou(cast.i32to64(result))), "IFileDialog.Show failed")
    ..
    ret try selectionFromDialog(a, dialog)
..

pub openFile(a allocator.Allocator, filters ptr, filterCount u64, defaultPath str, title str, parent ptr) !$str:
    ret try show(a, false, false, filters, filterCount, defaultPath, "", title, parent)
..

pub saveFile(a allocator.Allocator, filters ptr, filterCount u64, defaultPath str, defaultName str, title str, parent ptr) !$str:
    ret try show(a, true, false, filters, filterCount, defaultPath, defaultName, title, parent)
..

pub openDir(a allocator.Allocator, defaultPath str, title str, parent ptr) !$str:
    ret try show(a, false, true, none, 0, defaultPath, "", title, parent)
..
