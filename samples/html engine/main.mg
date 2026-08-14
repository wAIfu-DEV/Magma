mod main

# This project is a HTML engine implemented in Magma using Raylib

use "std:fmt" fmt
use "std:allocator" alc
use "std:cast" cast
use "std:file" file
use "std:footgun" footgun
use "std:fs" fs
use "std:heap" heap
use "std:list" list
use "std:raylib" rl
use "std:slices" slices
use "std:strings" strings
use "std:errors" errors

use "src/html.mg" html

const MAX_TASKS u64 = 128
const INPUT_CAPACITY u64 = 96
const SAVE_PATH str = "tasks.db"
const DEFAULT_PADDING i32 = 7
const DEFAULT_FONT_SIZE i32 = 15
const DEFAULT_FONT_SPACING f32 = 1.0
const SCROLL_IMPULSE f32 = 1200.0
const SCROLL_DAMPING f32 = 10.0
const SCROLL_STOP_SPEED f32 = 0.5

@platform("windows")
defaultFontPath() str:
    ret "C:/Windows/Fonts/arial.ttf"
..

@platform("darwin")
defaultFontPath() str:
    ret "/System/Library/Fonts/Supplemental/Arial.ttf"
..

@platform("linux", "freebsd", "netbsd", "openbsd")
defaultFontPath() str:
    # DejaVu Sans is the usual metric-compatible browser sans-serif fallback.
    ret "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf"
..

i32ToF32(value i32) f32:
    # SAFETY: this audited intrinsic performs the language's numeric conversion.
    unsafe:
        llvm "%result = sitofp i32 %value to float\n"
        llvm "ret float %result\n"
    ..
..

f32ToI32(value f32) i32:
    # SAFETY: this audited intrinsic performs the language's numeric conversion.
    unsafe:
        llvm "%result = fptosi float %value to i32\n"
        llvm "ret i32 %result\n"
    ..
..

Layout(
    x i32
    y i32
    width i32
    height i32
    textX i32
    textY i32
    fontSize i32
    children list.List[Layout]
)

destr Layout.free() void:
    this.children.free()
..

layoutCleanup(a alc.Allocator, val $Layout) void:
    val.free()
..

getFile(a alc.Allocator, args str[]) !$file.File:
    if args.count() < 2:
        throw errors.invalidArgument("missing argument: .html file path")
    ..
    mainArg str
    bounded 2 <= args.count():
        mainArg = args[1]
    ..
    
    _v, foundErr := strings.find(mainArg, ".html")
    if foundErr.nok():
        throw errors.invalidArgument("invalid argument: expected .html file path")
    ..
    ret try file.open(a, mainArg, file.mode().read())
..

isHiddenTag(tag str) bool:
    ret strings.compare(tag, "head") || strings.compare(tag, "script")
..

isTextNode(elem html.Html) bool:
    ret elem.tag.countBytes() == 0
..

layoutElement(a alc.Allocator, font rl.Font, elem html.Html, x i32, y i32, width i32) !$Layout:
    layout $Layout = Layout(
        x = x,
        y = y,
        width = width,
        height = 0,
        textX = x,
        textY = y,
        fontSize = DEFAULT_FONT_SIZE,
        children = try list.new[Layout](a, layoutCleanup),
    )

    if isHiddenTag(elem.tag):
        ret move layout
    ..

    if isTextNode(elem):
        metrics := rl.measureTextEx(font, elem.text, i32ToF32(layout.fontSize), DEFAULT_FONT_SPACING)
        layout.height = f32ToI32(metrics.y)
        ret move layout
    ..

    # Empty elements do not generate a visible box yet.
    if elem.children.count() == 0:
        ret move layout
    ..

    childX := x + DEFAULT_PADDING
    childY := y + DEFAULT_PADDING
    childWidth := width - DEFAULT_PADDING * 2

    for i u64 = 0 to elem.children.count():
        child := try elem.children.get(i)
        childLayout := try layoutElement(a, font, child, childX, childY, childWidth)
        childY = childY + childLayout.height
        try layout.children.pushRight(move childLayout)
    ..

    layout.height = childY - y + DEFAULT_PADDING
    ret move layout
..

drawElement(a alc.Allocator, font rl.Font, elem html.Html, layout Layout) !void:
    #fmt.str(a, "DISPLAYED: ").str(elem.tag).str("\n").print()

    if layout.height == 0:
        ret
    ..

    if isTextNode(elem):
        position := rl.Vector2(x = i32ToF32(layout.textX), y = i32ToF32(layout.textY))
        rl.drawTextEx(font, elem.text, position, i32ToF32(layout.fontSize), DEFAULT_FONT_SPACING, rl.black())
        ret
    ..

    rl.drawRectangle(layout.x, layout.y, layout.width, layout.height, rl.white())

    for i u64 = 0 to elem.children.count():
        try drawElement(a, font, try elem.children.get(i), try layout.children.get(i))
    ..
..


pub main(args str[]) !void:
    a := heap.allocator()

    f := try getFile(a, args)
    freader := try f.reader()

    root := try html.parseHtml(a, freader)
    defer root.free()

    f.close()

    rl.initWindow(900, 680, "HTML Engine")
    defer rl.closeWindow()
    
    rl.setWindowState(rl.flagWindowResizable() | rl.flagVsyncHint())
    rl.setTargetFPS(120)

    # Load the same sans-serif family browsers traditionally use for their
    # default font. The atlas is generated after the graphics context exists.
    browserFont := rl.loadFontEx(defaultFontPath(), DEFAULT_FONT_SIZE, none, 0)
    defer rl.unloadFont(browserFont)

    scroll f32 = 0.0
    scrollVelocity f32 = 0.0
    contentHeight i32 = 0

    loop rl.windowShouldClose() == false:
        screenWidth := rl.screenWidth()
        screenHeight := rl.screenHeight()

        delta := rl.frameTime()
        wheel := rl.mouseWheelMove()
        scrollVelocity = scrollVelocity + wheel * SCROLL_IMPULSE
        scroll = scroll + scrollVelocity * delta

        # Delta-scaled drag keeps inertia consistent across refresh rates.
        drag f32 = 1.0
        drag = drag - SCROLL_DAMPING * delta
        if drag < 0.0:
            drag = 0.0
        ..
        scrollVelocity = scrollVelocity * drag
        if scrollVelocity * scrollVelocity < SCROLL_STOP_SPEED * SCROLL_STOP_SPEED:
            scrollVelocity = 0.0
        ..

        minScroll := screenHeight - contentHeight
        if minScroll > 0:
            minScroll = 0
        ..
        if scroll > 0.0:
            scroll = 0.0
            scrollVelocity = 0.0
        elif scroll < i32ToF32(minScroll):
            scroll = i32ToF32(minScroll)
            scrollVelocity = 0.0
        ..

        rl.beginDrawing()
        defer rl.endDrawing()

        rl.clearBackground(rl.white())

        layout := try layoutElement(a, browserFont, root, 0, f32ToI32(scroll), screenWidth)
        defer layout.free()
        contentHeight = layout.height
        try drawElement(a, browserFont, root, layout)
    ..
..
