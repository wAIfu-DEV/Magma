# `std/json`

## Example

```magma
object := try json.newObject(heap.allocator(), cleanupValue)
defer object.free()
try object.set("answer", json.numberInt(42))
value := try object.get("answer")
answer := try value.asInt()
```

In-memory JSON values and serialization. This module constructs and writes JSON; it does not parse JSON text.

## Types

- `Value(value u128, kind u8)` stores a tagged null, boolean, float, integer, borrowed string, object pointer, or array pointer.
- `Object(entries linear_map.LinearMap[Value])` maps string keys to values. Keys are copied by the underlying linear map; values are shallow/borrowed.
- `Array(allocator alc.Allocator, values arr.Array[Value])` stores shallow values.

## Value creation and access

- `pub null() Value`, `pub boolean(value bool) Value`, `pub numberFloat(value f64) Value`, and `pub numberInt(value i64) Value` construct scalar values.
- `pub string(value str) Value` constructs a value borrowing `value`.
- `pub object(value Object*) Value` and `pub array(value Array*) Value` borrow container pointers.
- `Value.asNull() !ptr`, `asBool() !bool`, `asFloat() !f64`, `asInt() !i64`, `asString() !str`, `asObject() !Object*`, and `asArray() !Array*` return the payload or `invalidType` when the tag differs.

## Containers

- `pub newObject(a alc.Allocator, cleanup ($Value) void) !$Object` and `pub newArray(a alc.Allocator, cleanup ($Value) void) !$Array` allocate empty containers and configure value cleanup.
- `Object.set(key str, value $Value) !void`, `get(key str) !Value`, `delete(key str) !void`, `take(key str) !$Value`, and `count() u64` manage entries. `set` takes the value; `take` transfers a removed value without cleanup.
- `Object.free() void` is a `destr` method that frees copied keys and map storage, not nested values.
- `Array.append(value $Value) !void` takes and appends a value; `count() u64` returns its count.
- `Array.free() void` is a `destr` method that frees array storage, not nested values.

## Serialization

- `Value.write(w writer.Writer, precision u64) !void`, `Object.write(...)`, and `Array.write(...)` emit compact JSON. `precision` controls digits after the decimal point for floats; non-finite floats fail.
- `writeEscaped`, `finite`, `writeObject`, `writeArray`, and `writeValue` are internal serialization helpers.
