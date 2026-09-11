mod html

use "std:allocator" as alc
use "std:list" as list
use "std:linear_map" as linear_map
use "std:buffered" as buffered
use "std:reader" as reader
use "std:strings" as strings
use "std:slices" as slices
use "std:errors" as errors
use "std:builder" as builder
use "std:footgun" as footgun

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
        this.tag.free()
        this.text.free()
        this.attributes.free()
        this.children.free()
    ..
..

isVoidElement(tag str) bool:
    # HTML void elements never have end tags. Keep this centralized so the
    # tree builder and a future tokenizer can share the same HTML vocabulary.
    ret strings.compare(tag, "area") || strings.compare(tag, "base") || strings.compare(tag, "br") || strings.compare(tag, "col") || strings.compare(tag, "embed") || strings.compare(tag, "hr") || strings.compare(tag, "img") || strings.compare(tag, "input") || strings.compare(tag, "link") || strings.compare(tag, "meta") || strings.compare(tag, "param") || strings.compare(tag, "source") || strings.compare(tag, "track") || strings.compare(tag, "wbr") || strings.compare(tag, "!doctype")
..
isWhiteSpace(char u8) bool:
    s := " \t\n\r"
    _v, e := strings.findByte(s, char)
    ret e.ok()
..

htmlCleanup(val $Html) void:
    view := val.attributes.valuesView()
    for i u64 = 0 to view.count():
        # SAFETY: valuesView exposes the map-owned values, and cleanup uniquely
        # destroys each bounded entry before destroying the map storage.
        unsafe:
            view[i].free()
        ..
    ..
    val.free()
..

pub newScanner(a alc.Allocator, r reader.Reader) !$Scanner:
    sc $Scanner
    sc.allocator = a
    sc.reader = r

    sc.byteView = try slices.alloc[u8](1)
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
    this.allocator.free(slices.toPtr(this.byteView))
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

Scanner.ended() bool:
    ret this.atEnd
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

consumeComment(sc Scanner*) !void:
    previous2 u8 = 0
    previous1 u8 = 0
    loop sc.ended() == false:
        current := try sc.consume()
        if previous2 == strings.byteAt("-", 0) && previous1 == strings.byteAt("-", 0) && current == strings.byteAt(">", 0):
            ret
        ..
        previous2 = previous1
        previous1 = current
    ..
    throw errors.failure("unterminated HTML comment")
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
        attributes = try linear_map.new[str](none),
        children = try list.new[Html](a, htmlCleanup),
        allocator = a,
    )
    # give tag/text real owned (allocated) empty strings, mirroring how
    # valueless attributes use strings.alloc(0) instead of a "" literal --
    # this is needed since Html.free() unconditionally calls .free() on them
    element.tag = try strings.alloc(0)
    element.text = try strings.alloc(0)

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
                bld $builder.Builder = try builder.new()
                defer bld.free()

                loop isWhiteSpace(sc.peek()) == false && sc.peek() != strings.byteAt(">", 0):
                    try bld.addByte(try sc.consume())
                ..

                rawCloseTag $str = try bld.build()
                defer rawCloseTag.free()
                closeTag $str = try strings.toLower(rawCloseTag)
                defer closeTag.free()

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
            bld $builder.Builder = try builder.new()
            defer bld.free()

            # add tag name
            # TODO: check for alphabetic
            loop isWhiteSpace(sc.peek()) == false && sc.peek() != strings.byteAt(">", 0) && sc.peek() != strings.byteAt("/", 0):
                try bld.addByte(try sc.consume())
            ..
            rawTag $str = try bld.build()
            defer rawTag.free()

            tmp := try strings.toLower(rawTag)
            element.tag.free()
            element.tag = move tmp
        ..

        # Comments are tokens, not elements. Represent them as an empty node so
        # callers retain their simple one-result parser contract while layout
        # naturally ignores them.
        if strings.compare(element.tag, "!--"):
            try consumeComment(sc)
            ret move element
        ..
        
        # skip white space
        loop isWhiteSpace(sc.peek()):
            try sc.consume()
        ..

        # attribute sink
        explicitSelfClosing bool = false
        loop sc.peek() != strings.byteAt(">", 0):
            if sc.ended():
                throw errors.failure("unexpected end of input in start tag")
            ..
            if sc.peek() == strings.byteAt("/", 0):
                try sc.consume()
                explicitSelfClosing = true
                loop isWhiteSpace(sc.peek()):
                    try sc.consume()
                ..
                if sc.peek() != strings.byteAt(">", 0):
                    throw errors.failure("malformed self-closing tag")
                ..
                break
            ..
            # attribute name
            bld $builder.Builder = try builder.new()
            defer bld.free()

            # TODO: check for alphabetic
            loop isWhiteSpace(sc.peek()) == false && sc.peek() != strings.byteAt("=", 0) && sc.peek() != strings.byteAt(">", 0):
                try bld.addByte(try sc.consume())
            ..
            attrName := try bld.build()

            defer attrName.free()

            # whitespace and equal
            loop isWhiteSpace(sc.peek()):
                try sc.consume()
            ..

            if sc.peek() == strings.byteAt("=", 0):
                try sc.consume()
            else:
                # valueless attribute
                try element.attributes.set(attrName, try strings.alloc(0))
                continue
            ..

            loop isWhiteSpace(sc.peek()):
                try sc.consume()
            ..

            # attribute value
            quoteChar u8 = 0
            if sc.peek() == strings.byteAt("\"", 0):
                quoteChar = try sc.consume()
            elif sc.peek() == strings.byteAt("'", 0):
                quoteChar = try sc.consume()
            else:
                # HTML permits unquoted values up to whitespace or '>'.
                bld2 $builder.Builder = try builder.new()
                defer bld2.free()

                loop isWhiteSpace(sc.peek()) == false && sc.peek() != strings.byteAt(">", 0):
                    try bld2.addByte(try sc.consume())
                ..
                attrVal := try bld2.build()
                
                try element.attributes.set(attrName, move attrVal)
                loop isWhiteSpace(sc.peek()):
                    try sc.consume()
                ..
                continue
            ..
            
            bld2 $builder.Builder = try builder.new()
            defer bld2.free()

            loop sc.peek() != quoteChar:
                try bld2.addByte(try sc.consume())
            ..
            attrVal := try bld2.build()

            # end of value
            if sc.peek() == quoteChar:
                try sc.consume()
                try element.attributes.set(attrName, move attrVal)
            else:
                attrVal.free()
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

            if explicitSelfClosing || isVoidElement(element.tag):
                ret move element
            ..
        ..

        # children sink
        loop true:
            if sc.ended():
                throw errors.failure("unexpected end of input before closing tag")
            ..
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
        bld $builder.Builder = try builder.new()
        defer bld.free()

        loop sc.ended() == false && sc.peek() != strings.byteAt("<", 0):
            try bld.addByte(try sc.consume())
        ..


        tmp := try bld.build()
        element.text.free()
        element.text = move tmp
    ..

    ret move element
..
