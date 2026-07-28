mod main
Duo[A, B](first A, second B)
Duo[A, B].choose[C, D](first C, second D) D: ret second ..
main() void:
    duo := Duo[u64, bool](first=1, second=false)
    chosen bool = duo.choose[u8, bool](2, true)
..
