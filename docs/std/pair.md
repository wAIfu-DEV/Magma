# `std/pair`

`Pair[A, B]` is a generic two-value product with public `first A` and `second B`
fields. `new[A, B](first, second)` constructs one. It is useful where Magma's
two-result syntax is unavailable because that syntax is reserved for throwing
calls.
