mod duplex
# Type-erased bidirectional byte streams supporting reads and writes.

use "std:slices" slices
use "std:errors" errors
use "std:writer" writer
use "std:reader" reader

pub proto Duplex impl writer.Writer reader.Reader(
    write(bytes str) !u64
    readRaw(buff u8[], nBytes u64) !u64
)

Duplex.writer() writer.Writer:
    ret this.proto()
..

Duplex.reader() reader.Reader:
    ret this.proto()
..

Duplex.readToBuff(buff u8[], nBytes u64) !u64:
    if slices.count(buff) < nBytes:
        throw errors.invalidArgument("would overflow")
    ..
    readCount := try this.readRaw(buff, nBytes)
    if readCount > nBytes:
        throw errors.failure("duplex returned more bytes than requested")
    ..
    ret readCount
..
