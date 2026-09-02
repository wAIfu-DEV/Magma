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
use "src/element_defaults.mg" elem_defaults

const MAX_TASKS u64 = 128
const INPUT_CAPACITY u64 = 96
const SAVE_PATH str = "tasks.db"
const DEFAULT_PADDING i32 = 7
const DEFAULT_FONT_SIZE i32 = 15
const DEFAULT_FONT_SPACING f32 = 1.0
const TEXT_TOP_OFFSET i32 = 4
const SCROLL_IMPULSE f32 = 1200.0
const SCROLL_DAMPING f32 = 10.0
const SCROLL_STOP_SPEED f32 = 0.5

FontSet(
    body rl.Font
    h3 rl.Font
    h2 rl.Font
    h1 rl.Font
)

destr FontSet.free() void:
    rl.unloadFont(this.body)
    rl.unloadFont(this.h3)
    rl.unloadFont(this.h2)
    rl.unloadFont(this.h1)
..

fontFor(fonts FontSet*, fontSize i32) rl.Font:
    if fontSize == 30:
        ret fonts.h1
    elif fontSize == 24:
        ret fonts.h2
    elif fontSize == 19:
        ret fonts.h3
    ..
    ret fonts.body
..

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
    ret "/usr/share/fonts/TTF/DejaVuSans.ttf"
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

u64ToI32(value u64) i32:
    # Layout collections are bounded by available memory and screen geometry.
    unsafe:
        llvm "%result = trunc i64 %value to i32\n"
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
    color rl.Color
    background rl.Color
    borderWidth i32
    marginTop i32
    marginBottom i32
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
    ret try file.open(mainArg, file.mode().read())
..

isTextNode(elem html.Html) bool:
    ret elem.tag.countBytes() == 0
..

visibleChildCount(elem html.Html) u64:
    # Closed details exposes only its summary. The parser keeps the remaining
    # subtree so toggling can later become a state-only layout invalidation.
    if strings.compare(elem.tag, "details"):
        _open, e := elem.attributes.get("open")
        if e.nok() && elem.children.count() > 1:
            ret 1
        ..
    ..
    ret elem.children.count()
..

isFlowSpace(byte u8) bool:
    ret byte == strings.byteAt(" ", 0) || byte == strings.byteAt("\t", 0) || byte == strings.byteAt("\n", 0) || byte == strings.byteAt("\r", 0)
..

skipFlowSpace(text str, start u64) u64:
    cursor := start
    loop cursor < text.countBytes() && isFlowSpace(strings.byteAt(text, cursor)):
        cursor = cursor + 1
    ..
    ret cursor
..

nextLineEnd(a alc.Allocator, fonts FontSet*, text str, start u64, width i32, fontSize i32) !u64:
    font := fontFor(fonts, fontSize)
    cursor := start
    lastBreak := start
    loop cursor < text.countBytes():
        cursor = cursor + 1
        if isFlowSpace(strings.byteAt(text, cursor - 1)):
            lastBreak = cursor - 1
        ..
        candidate := try strings.substring(text, start, cursor)
        metrics := rl.measureTextEx(font, candidate, i32ToF32(fontSize), DEFAULT_FONT_SPACING)
        candidate.free(a)
        if metrics.x > i32ToF32(width):
            if lastBreak > start:
                ret lastBreak
            ..
            if cursor > start + 1:
                ret cursor - 1
            ..
            ret cursor
        ..
    ..
    ret cursor
..

wrappedTextHeight(a alc.Allocator, fonts FontSet*, text str, width i32, fontSize i32) !i32:
    if text.countBytes() == 0:
        ret 0
    ..
    lines i32 = 0
    cursor u64 = 0
    loop cursor < text.countBytes():
        cursor = skipFlowSpace(text, cursor)
        if cursor >= text.countBytes():
            break
        ..
        cursor = try nextLineEnd(a, fonts, text, cursor, width, fontSize)
        lines = lines + 1
    ..
    ret lines * (fontSize + 3)
..

drawWrappedText(a alc.Allocator, fonts FontSet*, text str, x i32, y i32, width i32, fontSize i32, color rl.Color) !void:
    font := fontFor(fonts, fontSize)
    cursor u64 = 0
    lineY := y
    loop cursor < text.countBytes():
        cursor = skipFlowSpace(text, cursor)
        if cursor >= text.countBytes():
            break
        ..
        lineEnd := try nextLineEnd(a, fonts, text, cursor, width, fontSize)
        line := try strings.substring(text, cursor, lineEnd)
        # Raylib positions text from the font atlas' top edge, whose glyphs in
        # the browser fonts sit slightly above our CSS-like line box.
        position := rl.Vector2(x=i32ToF32(x), y=i32ToF32(lineY + TEXT_TOP_OFFSET))
        rl.drawTextEx(font, line, position, i32ToF32(fontSize), DEFAULT_FONT_SPACING, color)
        line.free(a)
        cursor = lineEnd
        lineY = lineY + fontSize + 3
    ..
..

isInlineDisplay(display u8) bool:
    ret display == elem_defaults.DISPLAY_INLINE || display == elem_defaults.DISPLAY_INLINE_BLOCK
..

layoutElement(a alc.Allocator, sheet elem_defaults.StyleSheet*, fonts FontSet*, elem html.Html, inherited elem_defaults.Style, x i32, y i32, width i32) !$Layout:
    style := sheet.styleFor(elem.tag)
    if isTextNode(elem):
        # Text inherits text properties, not its parent's box model. Copying
        # margins here would apply the element's margins a second time.
        style.fontSize = inherited.fontSize
        style.color = inherited.color
    ..
    resolvedWidth := width - style.marginLeft - style.marginRight
    if style.preferredWidth > 0 && style.preferredWidth < resolvedWidth:
        resolvedWidth = style.preferredWidth
    ..
    layout $Layout = Layout(
        x = x + style.marginLeft,
        y = y + style.marginTop,
        width = resolvedWidth,
        height = 0,
        textX = x + style.marginLeft,
        textY = y + style.marginTop,
        fontSize = style.fontSize,
        color = style.color,
        background = style.background,
        borderWidth = style.borderWidth,
        marginTop = style.marginTop,
        marginBottom = style.marginBottom,
        children = try list.new[Layout](a, layoutCleanup),
    )

    if style.display == elem_defaults.DISPLAY_NONE:
        ret move layout
    ..

    if isTextNode(elem):
        layout.height = try wrappedTextHeight(a, fonts, elem.text, layout.width, layout.fontSize)
        metrics := rl.measureTextEx(fontFor(fonts, layout.fontSize), elem.text, i32ToF32(layout.fontSize), DEFAULT_FONT_SPACING)
        measuredWidth := f32ToI32(metrics.x) + 1
        if measuredWidth < layout.width:
            layout.width = measuredWidth
        ..
        ret move layout
    ..

    # Replaced/form elements still generate a useful box without child text.
    childCount := visibleChildCount(elem)
    if childCount == 0:
        if style.borderWidth > 0:
            layout.height = style.fontSize + style.paddingTop + style.paddingBottom + style.borderWidth * 2
        ..
        ret move layout
    ..

    childX := layout.x + style.paddingLeft + style.borderWidth
    childY := layout.y + style.paddingTop + style.borderWidth
    childWidth := layout.width - style.paddingLeft - style.paddingRight - style.borderWidth * 2

    # Table rows establish a horizontal formatting context. Column spanning
    # and intrinsic sizing can later replace this equal-share policy without
    # changing the surrounding block layout.
    if style.display == elem_defaults.DISPLAY_TABLE_ROW && childCount > 0:
        cellWidth := childWidth / u64ToI32(childCount)
        cellX := childX
        rowHeight i32 = 0
        for i u64 = 0 to childCount:
            child := try elem.children.get(i)
            childLayout := try layoutElement(a, sheet, fonts, child, style, cellX, childY, cellWidth)
            if childLayout.height > rowHeight:
                rowHeight = childLayout.height
            ..
            cellX = cellX + cellWidth
            try layout.children.pushRight(move childLayout)
        ..
        layout.height = rowHeight + style.paddingTop + style.paddingBottom + style.borderWidth * 2
        ret move layout
    ..

    inlineX := childX
    lineHeight i32 = 0
    maxLineWidth i32 = 0
    for i u64 = 0 to childCount:
        child := try elem.children.get(i)
        childStyle := sheet.styleFor(child.tag)
        childInline := isTextNode(child) || isInlineDisplay(childStyle.display)
        if childInline:
            remainingWidth := childX + childWidth - inlineX
            preferredOuterWidth := childStyle.marginLeft + childStyle.preferredWidth + childStyle.marginRight
            if childStyle.preferredWidth > 0 && preferredOuterWidth > remainingWidth && inlineX > childX:
                childY = childY + lineHeight
                inlineX = childX
                lineHeight = 0
                remainingWidth = childWidth
            ..
            if remainingWidth <= 0:
                childY = childY + lineHeight
                inlineX = childX
                lineHeight = 0
                remainingWidth = childWidth
            ..
            childLayout := try layoutElement(a, sheet, fonts, child, style, inlineX, childY, remainingWidth)
            outerWidth := childStyle.marginLeft + childLayout.width + childStyle.marginRight
            inlineX = inlineX + outerWidth
            outerHeight := childLayout.marginTop + childLayout.height + childLayout.marginBottom
            if outerHeight > lineHeight:
                lineHeight = outerHeight
            ..
            usedWidth := inlineX - childX
            if usedWidth > maxLineWidth:
                maxLineWidth = usedWidth
            ..
            try layout.children.pushRight(move childLayout)
            continue
        ..

        if inlineX > childX:
            childY = childY + lineHeight
            inlineX = childX
            lineHeight = 0
        ..
        childLayout := try layoutElement(a, sheet, fonts, child, style, childX, childY, childWidth)
        childY = childY + childLayout.marginTop + childLayout.height + childLayout.marginBottom
        try layout.children.pushRight(move childLayout)
    ..

    if inlineX > childX:
        childY = childY + lineHeight
    ..

    layout.height = childY - layout.y + style.paddingBottom + style.borderWidth
    if isInlineDisplay(style.display) && style.preferredWidth == 0:
        intrinsicWidth := maxLineWidth + style.paddingLeft + style.paddingRight + style.borderWidth * 2
        if intrinsicWidth < layout.width:
            layout.width = intrinsicWidth
        ..
    ..
    ret move layout
..

drawElement(a alc.Allocator, fonts FontSet*, elem html.Html, layout Layout) !void:
    #fmt.str(a, "DISPLAYED: ").str(elem.tag).str("\n").print()

    if layout.height == 0:
        ret
    ..

    if isTextNode(elem):
        try drawWrappedText(a, fonts, elem.text, layout.textX, layout.textY, layout.width, layout.fontSize, layout.color)
        ret
    ..

    if layout.background.a > 0:
        rl.drawRectangle(layout.x, layout.y, layout.width, layout.height, layout.background)
    ..
    if layout.borderWidth > 0:
        rl.drawRectangleLines(layout.x, layout.y, layout.width, layout.height, rl.Color(r=128,g=128,b=128,a=255))
    ..
    if strings.compare(elem.tag, "li"):
        rl.drawCircle(layout.x - 9, layout.y + layout.fontSize / 2, 2.5, layout.color)
    ..

    for i u64 = 0 to visibleChildCount(elem):
        try drawElement(a, fonts, try elem.children.get(i), try layout.children.get(i))
    ..
..


pub main(args str[]) !void:
    a := heap.allocator()

    styleSheet := try elem_defaults.new()
    defer styleSheet.free()

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
    browserFonts := FontSet(
        body=rl.loadFontEx(defaultFontPath(), DEFAULT_FONT_SIZE, none, 0),
        h3=rl.loadFontEx(defaultFontPath(), 19, none, 0),
        h2=rl.loadFontEx(defaultFontPath(), 24, none, 0),
        h1=rl.loadFontEx(defaultFontPath(), 30, none, 0),
    )
    defer browserFonts.free()

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

        rootStyle := styleSheet.styleFor(root.tag)
        layout := try layoutElement(a, addrof styleSheet, addrof browserFonts, root, rootStyle, 0, f32ToI32(scroll), screenWidth)
        defer layout.free()
        contentHeight = layout.height
        try drawElement(a, addrof browserFonts, root, layout)
    ..
..
