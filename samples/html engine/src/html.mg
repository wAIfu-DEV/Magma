mod html

use "std:allocator" alc
use "std:list" list
use "std:linear_map" linear_map
use "std:buffered" buffered
use "std:reader" reader
use "std:strings" strings
use "std:slices" slices
use "std:errors" errors
use "std:builder" builder
use "std:footgun" footgun

pub Html(
    tag str
    text str
    attributes linear_map.LinearMap[str]
    children list.List[Html]
    allocator alc.Allocator
)

destr Html.free() void:
    # SAFETY: Html uniquely owns all four destructible fields; its destructor
    # consumes each field exactly once before the aggregate becomes inactive.
    unsafe:
        this.tag.free(this.allocator)
        this.text.free(this.allocator)
        this.attributes.free()
        this.children.free()
    ..
..

isSelfClosing(tag str) bool:
    ret strings.compare(tag, "!doctype") || strings.compare(tag, "meta") || strings.compare(tag, "link") || strings.compare(tag, "input")
..
isWhiteSpace(char u8) bool:
    s := " \t\n\r"
    _v, e := strings.findByte(s, char)
    ret e.ok()
..

htmlCleanup(a alc.Allocator, val $Html) void:
    view := val.attributes.valuesView()
    for i u64 = 0 to view.count():
        # SAFETY: valuesView exposes the map-owned values, and cleanup uniquely
        # destroys each bounded entry before destroying the map storage.
        unsafe:
            view[i].free(val.allocator)
        ..
    ..
    val.free()
..

pub newScanner(a alc.Allocator, r reader.Reader) !$Scanner:
    sc $Scanner
    sc.allocator = a
    sc.reader = r

    sc.byteView = try slices.alloc[u8](a, 1)
    readCount := try sc.reader.readToBuff(sc.byteView, 1)
    sc.atEnd = readCount == 0

    sc.initialized = true
    ret sc
..

pub Scanner(
    allocator alc.Allocator
    reader reader.Reader
    byteView u8[]
    initialized bool
    atEnd bool
)

destr Scanner.close() void:
    slices.free(this.allocator, this.byteView)
    this.initialized = false
..

Scanner.peek() u8:
    if this.initialized == false || this.atEnd:
        ret 0
    ..
    bounded 1 <= this.byteView.count():
        ret this.byteView[0]
    ..
    ret 0
..

Scanner.consume() !u8:
    if this.initialized == false:
        throw errors.invalidArgument("scanner is not initialized")
    ..
    if this.atEnd:
        throw errors.outOfBounds("end of input")
    ..
    byte u8
    bounded 1 <= this.byteView.count():
        byte = this.byteView[0]
    ..
    readCount := try this.reader.readToBuff(this.byteView, 1)
    this.atEnd = readCount == 0
    ret byte
..

pub parseHtml(a alc.Allocator, r reader.Reader) !$Html:
    buff := try buffered.readerBuffered(r)
    defer buff.close()
    # Reader.reader() borrows its receiver, so create the interface from the
    # parseHtml-local buffer that remains alive and unmoved for the full parse.
    sc $Scanner = try newScanner(a, buff.reader())
    defer sc.close()

    # A doctype is a document declaration, not the document's root element.
    # Consume any leading doctype declarations and return the first real node
    # (normally <html>) to the renderer.
    loop true:
        root := try parseWithScanner(a, addrof sc, none)
        if strings.compare(root.tag, "!doctype"):
            root.free()
            continue
        ..
        ret move root
    ..
..


# The returned tree owns freshly allocated strings/containers and stores no
# scanner or parent pointer; both pointer arguments are call-duration borrows.
@no_retain
pub parseWithScanner(a alc.Allocator, sc Scanner*, parent Html*) !$Html:
    element $Html = Html(
        tag = "",
        text = "",
        attributes = try linear_map.new[str](a, none),
        children = try list.new[Html](a, htmlCleanup),
        allocator = a,
    )
    # give tag/text real owned (allocated) empty strings, mirroring how
    # valueless attributes use strings.alloc(a, 0) instead of a "" literal --
    # this is needed since Html.free() unconditionally calls .free() on them
    element.tag = try strings.alloc(a, 0)
    element.text = try strings.alloc(a, 0)

    onerror element.free()

    # skip leading white space
    loop isWhiteSpace(sc.peek()):
        try sc.consume()
    ..

    # start parsing an element
    if sc.peek() == strings.byteAt("<", 0):
        try sc.consume() # consume

        # closing tag: consume it fully (don't leave "/tagname>" sitting in
        # the stream) and signal "no more children" to the caller
        if sc.peek() == strings.byteAt("/", 0):
            try sc.consume() # consume /

            if true:
                bld $builder.Builder = try builder.new(a)
                defer bld.free()

                loop isWhiteSpace(sc.peek()) == false && sc.peek() != strings.byteAt(">", 0):
                    try bld.addByte(try sc.consume())
                ..

                rawCloseTag $str = try bld.build()
                defer rawCloseTag.free(a)
                closeTag $str = try strings.toLower(a, rawCloseTag)
                defer closeTag.free(a)

                if parent != none && strings.compare(closeTag, parent.tag) == false:
                    throw errors.failure("mismatched closing tag")
                ..

                loop isWhiteSpace(sc.peek()):
                    try sc.consume()
                ..

                if sc.peek() != strings.byteAt(">", 0):
                    #element.free()
                    throw errors.failure("malformed closing tag")
                ..
                try sc.consume() # consume >
            ..

            if parent == none:
                throw errors.failure("stray closing tag at top level")
            ..
            throw errors.outOfBounds("end of children")
        ..

        # tag sink
        if true:
            bld $builder.Builder = try builder.new(a)
            defer bld.free()

            # add tag name
            # TODO: check for alphabetic
            loop isWhiteSpace(sc.peek()) == false && sc.peek() != strings.byteAt(">", 0) && sc.peek() != strings.byteAt("/", 0):
                try bld.addByte(try sc.consume())
            ..
            rawTag $str = try bld.build()
            defer rawTag.free(a)

            tmp := try strings.toLower(a, rawTag)
            element.tag.free(a)
            element.tag = move tmp
        ..
        
        # skip white space
        loop isWhiteSpace(sc.peek()):
            try sc.consume()
        ..

        # check if crime against humanity
        if sc.peek() == strings.byteAt("/", 0):
            throw errors.failure("invalid closing slash")
        ..

        # attribute sink
        loop sc.peek() != strings.byteAt(">", 0):
            # attribute name
            bld $builder.Builder = try builder.new(a) 

            # TODO: check for alphabetic
            loop isWhiteSpace(sc.peek()) == false && sc.peek() != strings.byteAt("=", 0) && sc.peek() != strings.byteAt(">", 0):
                try bld.addByte(try sc.consume())
            ..
            attrName := try bld.build()
            bld.free()

            defer attrName.free(a)

            # whitespace and equal
            loop isWhiteSpace(sc.peek()):
                try sc.consume()
            ..

            if sc.peek() == strings.byteAt("=", 0):
                try sc.consume()
            else:
                # valueless attribute
                try element.attributes.set(attrName, try strings.alloc(a, 0))
                continue
            ..

            loop isWhiteSpace(sc.peek()):
                try sc.consume()
            ..

            quoteChar u8 = 0

            # attribute value
            if sc.peek() == strings.byteAt("\"", 0):
                quoteChar = try sc.consume()
            elif sc.peek() == strings.byteAt("'", 0):
                quoteChar = try sc.consume()
            else:
                throw errors.failure("expected double quote or single quote")
            ..
            
            bld2 $builder.Builder = try builder.new(a)
            loop sc.peek() != quoteChar:
                try bld2.addByte(try sc.consume())
            ..
            attrVal := try bld2.build()
            bld2.free()

            # end of value
            if sc.peek() == quoteChar:
                try sc.consume()
                try element.attributes.set(attrName, move attrVal)
            else:
                attrVal.free(a)
                throw errors.failure("expected end of quoted string")
            ..

            # skip white space
            loop isWhiteSpace(sc.peek()):
                try sc.consume()
            ..
        ..

        # check if first byte is closing bracket
        if sc.peek() == strings.byteAt(">", 0):
            try sc.consume()

            if isSelfClosing(element.tag):
                ret move element
            ..
        ..

        # children sink
        loop true:
            child, e := parseWithScanner(a, sc, addrof element)
            if e.nok():
                if errors.hasCode(e, errors.ERR_OUT_OF_BOUNDS):
                    break
                ..
                throw e
            ..
            try element.children.pushRight(move child)
        ..
        ret move element
    ..

    # text node: everything up to the next '<' (leading whitespace was
    # already skipped above, so this only fires on non-whitespace content
    # or interior whitespace between the first char and a following '<')
    if true:
        bld $builder.Builder = try builder.new(a)
        defer bld.free()

        loop sc.peek() != strings.byteAt("<", 0):
            try bld.addByte(try sc.consume())
        ..


        tmp := try bld.build()
        element.text.free(a)
        element.text = move tmp
    ..

    ret move element
..
