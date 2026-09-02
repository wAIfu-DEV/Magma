# `std/json`

## Example

```magma
document := try json.parse(source)
defer document.free()

object := try document.asObject()
try object.setBool("processed", true)
name := try document.get("name").asString()
try document.write(output)
```

Parsing, value construction, lookup, mutation, ownership, and serialization of
JSON values. Public operations pass values rather than pointers.

## Value model

- `Value` is the only owning public JSON type. Its `free()` destructor
  recursively releases strings, arrays, and objects.
- `Object` and `Array` are borrowed, copyable value views returned by
  `Value.asObject()` and `Value.asArray()`. Their internal storage is not exposed.
- `Object.get` and `Array.get` return borrowed `Value`s. `Object.take` returns
  `$Value` and transfers ownership to the caller.
- Containers consume inserted `$Value`s. Normal nesting uses
  `object.set("child", move child)` or `array.append(move child)`.
- Values have no runtime borrowed/owned or boxed/unboxed flags. Magma's
  ownership checker distinguishes borrowed `Value` returns from `$Value`
  transfers.

## Construction and access

- `object() !$Value` and `array() !$Value` create owned containers.
- `null() $Value`, `bool(value) $Value`, `numberInt(value) $Value`, and
  `numberFloat(value) $Value` create scalar values.
- `string(value) !$Value` copies text into an owned JSON string.
- `asNull() !void`, `asBool`, `asInt`, `asFloat`, `asString`, `asObject`, and `asArray`
  validate the kind and return `invalidType` on mismatch.

Object views provide `set`, `get`, `take`, `delete`, and `count`, plus
`setString`, `setInt`, and `setBool` conveniences. Array views provide `append`,
`get`, and `count`.

Generic object `get` and `set` are also forwarded by `Value`. Convenience
setters remain on the `Object` view:

```magma
document := try json.object()
defer document.free()

object := try document.asObject()
try object.setString("name", "Magma")
try object.setInt("version", 2)
name := try document.get("name").asString()

items := try json.array()
defer items.free()
array := try items.asArray()
try array.append(json.numberInt(10))
second := try array.get(0).asInt()
```

## Parsing

- `parse(source) !$Value` parses exactly one complete JSON text.
- Every JSON kind is accepted at the root.
- Parsing rejects trailing content and commas, malformed numbers, invalid UTF-8,
  invalid escapes, unescaped control characters, and unpaired UTF-16 surrogates.
- Duplicate object keys use last-value-wins semantics.
- Nesting is limited to 128 arrays or objects.

## Serialization

- `Value.write(writer)` emits compact JSON and uses six fractional digits for
  floating-point values.
- `Value.writeWithPrecision(writer, precision)` selects fractional precision.
- Object members are emitted in their current map order. Non-finite floats are
  rejected because JSON has no representation for them.
