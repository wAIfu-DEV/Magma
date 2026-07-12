mod duplex

use "writer.mg" wr
use "reader.mg" rd

Duplex(
    impl ptr
    fn_write (ptr, str) !u64,
    fn_read (ptr, u8[], u64) !u64,
)

pub new(impl ptr, writeFunc (ptr, str) !u64, readFunc (ptr, u8[], u64) !u64) Duplex:
    d Duplex
    d.impl = impl
    d.fn_write = writeFunc
    d.fn_read = readFunc
    ret d
..

Duplex.writer() wr.Writer:
    ret wr.new(this.impl, this.fn_write)
..

Duplex.reader() rd.Reader:
    ret rd.new(this.impl, this.fn_read)
..
