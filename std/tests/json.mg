mod main

use "std:allocator" allocator
use "std:cast" cast
use "std:errors" errors
use "std:heap" heap
use "std:json" json
use "std:memory" memory
use "std:slices" slices
use "std:strings" strings
use "std:writer" writer
use "std:footgun" footgun

Capture(
    data u8*
    count u64*
)

captureWrite(raw ptr, bytes str) !u64:
    output Capture* = raw
    count := strings.countBytes(bytes)
    countPtr u64* = output.count
    destination := cast.utop(cast.ptou(output.data) + *countPtr)
    memory.copy(strings.toPtr(bytes), destination, count)
    *countPtr = *countPtr + count
    ret count
..

render(a allocator.Allocator, value json.Value, precision u64) !$str:
    storage u8* = try a.allocT[u8](1024)
    defer a.free(storage)
    count u64 = 0
    output Capture
    output.data = storage
    output.count = addrof count
    sink := writer.new(addrof output, captureWrite)
    try value.write(sink, precision)
    view := strings.fromPtrNoCopy(output.data, count)
    ret try strings.copy(a, view)
..

pub main() !void:
    a allocator.Allocator = heap.allocator()

    nullValue := json.null()
    try nullValue.asNull()
    wrongBool bool, wrongType error = nullValue.asBool()
    if errors.code(wrongType) != 6:
        throw errors.failure("JSON accessor accepted the wrong type")
    ..

    truth := json.boolean(true)
    if try truth.asBool() == false:
        throw errors.failure("JSON bool round trip failed")
    ..
    integer := json.numberInt(-42)
    integerValue := try integer.asInt()
    integerFloat := try integer.asFloat()
    if integerValue != -42 || integerFloat != -42.0:
        throw errors.failure("JSON integer conversion failed")
    ..
    floating := json.numberFloat(12.75)
    floatValue := try floating.asFloat()
    floatInteger := try floating.asInt()
    if floatValue != 12.75 || floatInteger != 12:
        throw errors.failure("JSON float conversion failed")
    ..

    object := try json.newObject(a)
    defer object.free()
    objectValue := json.objectBorrowed(addrof object)
    if try objectValue.asObject() != addrof object:
        throw errors.failure("JSON object accessor changed")
    ..
    try object.set("answer", json.numberInt(41))
    try object.set("answer", json.numberInt(42))
    answer := try object.get("answer")
    answerValue := try answer.asInt()
    if answerValue != 42 || object.count() != 1:
        throw errors.failure("JSON object replacement failed")
    ..
    taken := try object.take("answer")
    takenValue := try taken.asInt()
    if takenValue != 42 || object.count() != 0:
        throw errors.failure("JSON object take failed")
    ..
    try object.set("temporary", json.boolean(false))
    try object.delete("temporary")
    if object.count() != 0:
        throw errors.failure("JSON object delete failed")
    ..

    array := try json.newArray(a)
    defer array.free()
    arrayValue := json.arrayBorrowed(addrof array)
    if try arrayValue.asArray() != addrof array:
        throw errors.failure("JSON array accessor or borrowing changed")
    ..
    borrowedArrayValue := arrayValue.borrowed()
    if try borrowedArrayValue.asArray() != addrof array:
        throw errors.failure("JSON array borrowing changed")
    ..
    specialBytes := array u8[5]
    specialBytes[0] = 34
    specialBytes[1] = 92
    specialBytes[2] = 10
    specialBytes[3] = 9
    specialBytes[4] = 1
    special := strings.fromPtrNoCopy(slices.toPtr(specialBytes), 5)
    specialValue := try json.stringCopy(a, special)
    specialRoundTrip := try specialValue.asString()
    if strings.countBytes(specialRoundTrip) != 5:
        throw errors.failure("JSON string payload round trip failed")
    ..
    try array.append(specialValue)
    try array.append(json.numberInt(-7))
    second := try array.get(1)
    secondNumber := try second.asInt()
    if array.count() != 2 || secondNumber != -7:
        throw errors.failure("JSON array access failed")
    ..
    missing json.Value, boundsErr error = array.get(2)
    if errors.code(boundsErr) != 2:
        throw errors.failure("JSON array accepted an out-of-bounds index")
    ..

    escaped := try render(a, json.arrayBorrowed(addrof array), 2)
    defer strings.free(a, escaped)
    escapedLength := strings.countBytes(escaped)
    if escapedLength != 21:
        throw errors.failure("JSON escaped output has the wrong length")
    ..
    expected := array u8[21]
    expected[0] = 91
    expected[1] = 34
    expected[2] = 92
    expected[3] = 34
    expected[4] = 92
    expected[5] = 92
    expected[6] = 92
    expected[7] = 110
    expected[8] = 92
    expected[9] = 116
    expected[10] = 92
    expected[11] = 117
    expected[12] = 48
    expected[13] = 48
    expected[14] = 48
    expected[15] = 49
    expected[16] = 34
    expected[17] = 44
    expected[18] = 45
    expected[19] = 55
    expected[20] = 93
    byteIndex u64 = 0
    while byteIndex < 21:
        if strings.byteAt(escaped, byteIndex) != expected[byteIndex]:
            throw errors.failure("JSON string escaping changed")
        ..
        byteIndex = byteIndex + 1
    ..

    nested := try json.newObject(a)
    try nested.set("ok", json.boolean(true))
    try object.set("items", json.arrayBorrowed(addrof array))
    try object.set("nested", json.objectOwned(addrof nested))
    footgun.drop[json.Object](nested)
    encoded := try render(a, json.objectBorrowed(addrof object), 2)
    defer strings.free(a, encoded)
    encodedLength := strings.countBytes(encoded)
    if encodedLength < 2 || strings.byteAt(encoded, 0) != 123 || strings.byteAt(encoded, encodedLength - 1) != 125:
        throw errors.failure("JSON nested object serialization changed")
    ..

    copied := try json.stringCopy(a, "owned")
    ownedText := try copied.asString()
    if strings.compare(ownedText, "owned") == false:
        throw errors.failure("JSON owned string copy failed")
    ..
    cleanup := try json.newArray(a)
    try cleanup.append(copied)

    borrowedValue := json.stringBorrowed("borrowed")
    if strings.compare(try borrowedValue.asString(), "borrowed") == false:
        cleanup.free()
        throw errors.failure("JSON borrowed string changed")
    ..
    transferredText := try strings.copy(a, "transferred")
    try cleanup.append(json.stringOwned(a, transferredText))

    child := try json.newArray(a)
    try child.append(json.boolean(true))
    try cleanup.append(json.arrayOwned(addrof child))
    footgun.drop[json.Array](child)
    cleanup.free()
..
