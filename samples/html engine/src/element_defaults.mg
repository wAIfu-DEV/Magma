mod elem_defaults

use "std:raylib" rl
use "std:allocator" alc
use "std:hash_map" hm
use "std:vector" vc

Style(
    color rl.Color
    fontSize i32
    padding vc.Vec4[i32]
)

const defaultStyle := Style(
    color = rl.Color(r=0,g=0,b=0,a=255)
    fontSize = 15
    padding = vc.Vec4[i32](i0=7,i1=7,i2=7,i3=7)
)

nameToId hm.HashMap[u32]

# Only register elements with special styling
const EL_H1 u32 = 0
const EL_SENTINEL u32 = 1

const defaults Style[] = array Style[EL_SENTINEL]

pub init(a alc.Allocator) !void:
    nameToId = try hm.new[u32](a, 32, none)

    for i := 0 to EL_SENTINEL:
        defaults[i] = defaultStyle
    ..

    nameToId.set("h1", EL_H1)
    defaults[EL_H1].fontSize = 30
..
