# `std/json`

## Example

```magma
object := try json.newObject(heap.allocator())
defer object.free()
try object.set("answer", json.numberInt(42))
value := try object.get("answer")
answer := try value.asInt()
```

In-memory JSON values and serialization. This module constructs and writes JSON; it does not parse JSON text.

## Types

- `Value(value u128, kind u8, owned bool, allocator alc.Allocator)` stores a
  tagged payload plus the ownership metadata needed to release owned strings or
  containers.
- `Object(entries linear_map.LinearMap[Value])` owns copied keys and uses the
  JSON value destructor for owned values.
- `Array(allocator alc.Allocator, values arr.Array[Value])` owns values marked
  as owned.

## Value creation and access

- `pub null() Value`, `pub bool(value bool) Value`, `pub numberFloat(value f64) Value`, and `pub numberInt(value i64) Value` construct scalar values.
- `stringBorrowed`, `objectBorrowed`, and `arrayBorrowed` construct borrowed
  values. `stringOwned`, `objectOwned`, and `arrayOwned` transfer ownership.
  `stringCopy(a, value) !$Value` allocates an owned copy.
- `Value.borrowed() Value` returns a non-owning view of any value.
- `Value.asNull() !ptr`, `asBool() !bool`, `asFloat() !f64`, `asInt() !i64`, `asString() !str`, `asObject() !Object*`, and `asArray() !Array*` return the payload or `invalidType` when the tag differs.

## Containers

- `pub newObject(a alc.Allocator) !$Object` and `pub newArray(a alc.Allocator) !$Array` allocate empty containers with JSON value cleanup configured internally.
- `Object.set(key str, value $Value) !void`, `get(key str) !Value`, `delete(key str) !void`, `take(key str) !$Value`, and `count() u64` manage entries. `set` takes the value; `take` transfers a removed value without cleanup.
- `Object.free() void` frees copied keys, owned values, and map storage.
- `Array.append(value $Value) !void` appends a value; `count() u64` returns its
  count and `get(index u64) !Value` borrows an indexed value.
- `Array.free() void` frees owned values and array storage.

## Serialization

- `Value.write(w writer.Writer, precision u64) !void`, `Object.write(...)`, and `Array.write(...)` emit compact JSON. `precision` controls digits after the decimal point for floats; non-finite floats fail.
- `writeEscaped`, `finite`, `writeObject`, `writeArray`, and `writeValue` are internal serialization helpers.
