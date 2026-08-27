mod main

accept64(value f64) f64:
    ret value
..

accept32(value f32) f32:
    ret value
..

main() void:
    inferred := 1.5
    difference := accept64(0.0 - 0.0)
    sum := accept32(1.25 + 2.5)
..
