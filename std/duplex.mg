mod duplex

use "writer.mg" wr
use "reader.mg" rd

Vtable(
    fn_write (ptr, str) !u64,
    fn_read (ptr, u8[], u64) !u64,
)

Duplex(
    impl ptr
    vtable Vtable*
)

pub new(impl ptr, vtable Vtable*) Duplex:
    ret Duplex(impl=impl, vtable=vtable)
..

Duplex.writer() wr.Writer:
    ret wr.new(this.impl, this.vtable.fn_write)
..

Duplex.reader() rd.Reader:
    ret rd.new(this.impl, this.vtable.fn_read)
..
