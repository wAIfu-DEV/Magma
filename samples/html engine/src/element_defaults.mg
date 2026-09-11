mod elem_defaults

# A small data-driven user-agent style sheet. A future CSS cascade can populate
# or replace this registry without changing the layout and paint passes.
use "std:raylib" as rl
use "std:hash_map" as hm

pub const DISPLAY_NONE u8 = 0
pub const DISPLAY_BLOCK u8 = 1
pub const DISPLAY_INLINE u8 = 2
pub const DISPLAY_TABLE u8 = 3
pub const DISPLAY_TABLE_ROW u8 = 4
pub const DISPLAY_TABLE_CELL u8 = 5
pub const DISPLAY_INLINE_BLOCK u8 = 6

pub Style(
    display u8
    color rl.Color
    background rl.Color
    fontSize i32
    marginTop i32
    marginRight i32
    marginBottom i32
    marginLeft i32
    paddingTop i32
    paddingRight i32
    paddingBottom i32
    paddingLeft i32
    borderWidth i32
    preferredWidth i32
)

pub StyleSheet(
    rules hm.HashMap[Style]
    fallback Style
)

destr StyleSheet.free() void:
    this.rules.free()
..

base(display u8) Style:
    ret Style(display=display, color=rl.Color(r=0,g=0,b=0,a=255), background=rl.Color(r=255,g=255,b=255,a=0), fontSize=15, marginTop=0, marginRight=0, marginBottom=0, marginLeft=0, paddingTop=0, paddingRight=0, paddingBottom=0, paddingLeft=0, borderWidth=0, preferredWidth=0)
..

paragraph() Style:
    style := base(DISPLAY_BLOCK)
    style.marginTop = 8
    style.marginBottom = 8
    ret style
..

add(sheet StyleSheet*, name str, style Style) !void:
    try sheet.rules.set(name, style)
..

addStructuralRules(sheet StyleSheet*, block Style, form Style) !void:
    try add(sheet, "html", block)
    body := block
    # Conventional browser user-agent style: body { margin: 8px; }
    body.marginTop = 8
    body.marginRight = 8
    body.marginBottom = 8
    body.marginLeft = 8
    try add(sheet, "body", body)
    try add(sheet, "header", block)
    try add(sheet, "main", block)
    try add(sheet, "footer", block)
    try add(sheet, "section", block)
    try add(sheet, "article", block)
    try add(sheet, "nav", block)
    try add(sheet, "thead", block)
    try add(sheet, "tbody", block)
    fieldset := form
    # The block children already provide their own vertical margins. Keeping
    # form padding here as well makes the fieldset look padded twice.
    fieldset.paddingTop = 0
    fieldset.paddingBottom = 0
    try add(sheet, "fieldset", fieldset)

    form.paddingTop = 0
    form.paddingBottom = 0
    form.paddingLeft = 0
    form.paddingRight = 0
    form.borderWidth = 0
    try add(sheet, "form", form)
..

addTextRules(sheet StyleSheet*, p Style) !void:
    try add(sheet, "p", p)
    try add(sheet, "summary", p)
    try add(sheet, "label", base(DISPLAY_INLINE))
    # A legend is a label for the fieldset, not a paragraph; paragraph margins
    # otherwise add a conspicuous gap above and below it.
    try add(sheet, "legend", base(DISPLAY_BLOCK))

    h1 := p
    h1.fontSize = 30
    h1.marginTop = 20
    h1.marginBottom = 12
    try add(sheet, "h1", h1)
    h2 := p
    h2.fontSize = 24
    h2.marginTop = 16
    try add(sheet, "h2", h2)
    h3 := p
    h3.fontSize = 19
    h3.marginTop = 12
    try add(sheet, "h3", h3)

    list := p
    list.paddingLeft = 28
    try add(sheet, "ul", list)
    try add(sheet, "ol", list)
    item := p
    item.marginTop = 2
    item.marginBottom = 2
    try add(sheet, "li", item)
    quote := p
    quote.marginLeft = 24
    quote.paddingLeft = 12
    quote.borderWidth = 1
    try add(sheet, "blockquote", quote)
    details := p
    details.paddingLeft = 8
    try add(sheet, "details", details)
..

addTableRules(sheet StyleSheet*, p Style) !void:
    table := p
    table.display = DISPLAY_TABLE
    table.borderWidth = 0
    try add(sheet, "table", table)
    try add(sheet, "tr", base(DISPLAY_TABLE_ROW))
    cell := base(DISPLAY_TABLE_CELL)
    cell.paddingTop = 5
    cell.paddingRight = 8
    cell.paddingBottom = 5
    cell.paddingLeft = 8
    cell.borderWidth = 1
    try add(sheet, "th", cell)
    try add(sheet, "td", cell)
..

addControlRules(sheet StyleSheet*, p Style) !void:
    control := base(DISPLAY_INLINE_BLOCK)
    control.paddingTop = 2
    control.paddingRight = 4
    control.paddingBottom = 2
    control.paddingLeft = 4
    control.borderWidth = 1
    control.preferredWidth = 220
    try add(sheet, "input", control)
    try add(sheet, "textarea", control)
    button := control
    button.preferredWidth = 0
    button.background = rl.Color(r=232,g=232,b=232,a=255)
    try add(sheet, "button", button)
..

pub new() !$StyleSheet:
    sheet $StyleSheet = StyleSheet(rules=try hm.new[Style](64, none), fallback=base(DISPLAY_INLINE))
    onerror sheet.free()

    block := base(DISPLAY_BLOCK)
    p := paragraph()
    form := p
    form.paddingTop = 8
    form.paddingRight = 8
    form.paddingBottom = 8
    form.paddingLeft = 8
    form.borderWidth = 1
    try addStructuralRules(addrof sheet, block, form)
    try addTextRules(addrof sheet, p)
    try addTableRules(addrof sheet, p)
    try addControlRules(addrof sheet, p)

    hidden := base(DISPLAY_NONE)
    try add(addrof sheet, "head", hidden)
    try add(addrof sheet, "script", hidden)
    try add(addrof sheet, "style", hidden)
    try add(addrof sheet, "meta", hidden)
    try add(addrof sheet, "link", hidden)
    try add(addrof sheet, "title", hidden)
    ret move sheet
..

StyleSheet.styleFor(tag str) Style:
    style, e := this.rules.get(tag)
    if e.ok():
        ret style
    ..
    ret this.fallback
..
