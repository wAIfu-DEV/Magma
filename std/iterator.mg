mod iterator

Iterator[T](
    impl ptr
    index u64

    fn_hasData (ptr, u64*) bool
    fn_next (ptr, u64*) !T
)

pub new[T](impl ptr, hasDataFunc (ptr, u64*) bool, nextFunc (ptr, u64*) !T) Iterator[T]:
    ret Iterator[T](impl=impl, index=0, fn_hasData=hasDataFunc, fn_next=nextFunc)
..

Iterator[T].hasData() bool:
    ret this.fn_hasData(this.impl, addrof this.index)
..

Iterator[T].next() !T:
    ret this.fn_next(this.impl, addrof this.index)
..
