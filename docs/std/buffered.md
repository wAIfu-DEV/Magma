# `std/buffered`

Buffered adapters over `std/writer.Writer` and `std/reader.Reader`. Both allocate an internal buffer, so they must be closed.

## Types

- `Writer` holds an underlying writer, raw buffer, buffer size, current position, and allocator.
- `Reader` holds an underlying reader, buffer, read position, filled byte count, allocator, and EOF state.

## Writer API

- `pub writerBuffered(a alc.Allocator, w writer.Writer) !$Writer` creates a buffered writer with the default buffer size.
- `Writer.flush() !u64` writes all pending bytes and returns the number written. On a partial write or error, unwritten bytes remain buffered.
- `Writer.writer() writer.Writer` returns a generic writer view backed by this object; the `Writer` must outlive the view.
- `Writer.close() !void` is a throwing `destr` method that flushes and frees the buffer.
- `bufferedWrite(bw Writer*, bytes str) !u64` is the internal adapter callback, buffering small writes and directly handling large ones.

## Reader API

- `pub readerBuffered(a alc.Allocator, r reader.Reader) !$Reader` creates a buffered reader.
- `Reader.fillBuffer() !bool` refills the buffer and reports whether data was obtained.
- `Reader.reader() reader.Reader` returns a generic reader view; the buffered reader must outlive it.
- `Reader.readLn(a alc.Allocator) !$str` reads through the next newline and returns an owned string without `\n`. It returns the final unterminated line at EOF.
- `Reader.close() void` is a `destr` method that frees the internal buffer.
- `bufferedRead(br Reader*, buff u8[], nBytes u64) !u64` is the internal reader callback.
- `resizeLineBuffer(a alc.Allocator, old u8*, newCapacity u64) !$u8*` is the checked internal growth helper used by `readLn`.
