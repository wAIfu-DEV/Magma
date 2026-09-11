mod string

use "std:allocator" as alc

pub String(
    s $str
)

String.view() str:
    ret this.s
..

destr String.free() void:
    this.s.free()
..

destr String.toStr() $str:
    ret move this.s
..

pub newA(a alc.Allocator, s $str) $String:
    ret String(s=move s)
..

pub new(s $str) $String:
    ret newA(ctx.alloc, move s)
..
