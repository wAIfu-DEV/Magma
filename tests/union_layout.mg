mod main

union Choice(
    Empty
    Small(value u8)
    Large(left u64, right u64)
)

union WideChoice(
    Small(value u8)
    Wide(value u128)
    Long(a u64, b u64, c u64)
)

readChoice(value Choice) u64:
    match value as selected:
    case Choice.Empty:
        ret 0
    case Choice.Small:
        ret selected.value
    case Choice.Large:
        ret selected.left + selected.right
    ..
    ret 0
..

main() !void:
    if sizeof Choice != 24:
        throw "wrong union size"
    ..
    if sizeof WideChoice != 48:
        throw "wrong wide union size"
    ..
    value := Choice.Large(left=17, right=29)
    match value as selected:
    case Choice.Empty:
        throw "wrong tag"
    case Choice.Small:
        throw "wrong tag"
    case Choice.Large:
        if selected.left != 17 || selected.right != 29:
            throw "wrong payload"
        ..
    ..
    if readChoice(value) != 46:
        throw "wrong union call"
    ..
    wide := WideChoice.Wide(value=123)
    match wide as selectedWide:
    case WideChoice.Small:
        throw "wrong wide tag"
    case WideChoice.Wide:
        if selectedWide.value != 123:
            throw "wrong wide payload"
        ..
    case WideChoice.Long:
        throw "wrong wide tag"
    ..
..
