mod string

use "std:allocator" alc

pub String(
    s $str
    allocator alc.Allocator
)

String.view() str:
    ret this.s
..

destr String.free() void:
    this.s.free(this.allocator)
..

destr String.toStr() $str:
    ret move this.s
..

pub newA(a alc.Allocator, s $str) $String:
    ret String(s=move s,allocator=a)
..

pub new(s $str) $String:
    ret newA(ctx.alloc, move s)
..