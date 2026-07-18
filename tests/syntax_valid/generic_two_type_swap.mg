mod main
Pair[A, B](left A, right B)
swap[A, B](pair Pair[A, B]) Pair[B, A]:
    ret Pair[B, A](left=pair.right, right=pair.left)
..
main() void:
    pair Pair[u64, bool] = Pair[u64, bool](left=1, right=true)
    swapped Pair[bool, u64] = swap[u64, bool](pair)
..
