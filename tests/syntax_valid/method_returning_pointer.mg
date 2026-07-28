mod main
Holder(value u8*)
Holder.get() u8*: ret this.value ..
pub main() void:
    holder := Holder(value=none)
    value u8* = holder.get()
..
