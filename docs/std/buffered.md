# `std/buffered`

## Example

```magma
output := try buffered.writerBuffered(heap.allocator(), rawWriter)
defer output.close()
try output.writer().writeAll("buffered output")
try output.flush()
```

Buffered adapters over `std/writer.Writer` and `std/reader.Reader`. Both allocate an internal buffer, so they must be closed.

## Types

- `Writer(underlying writer.Writer, buffer ptr, position u64, allocator alc.Allocator)` owns an 8 KiB buffer.
- `Reader(underlying reader.Reader, buffer u8*, position u64, filled u64, allocator alc.Allocator)` stores EOF in the high bit of `filled` and the valid-byte count in the remaining bits.

## Writer API

- `pub writerBuffered(a alc.Allocator, w writer.Writer) !$Writer` creates a buffered writer with the default buffer size.
- `Writer.flush() !u64` writes all pending bytes and returns the number written. On a partial write or error, unwritten bytes remain buffered.
- `Writer.writer() writer.Writer` returns a generic writer view backed by this object; the `Writer` must outlive the view.
- `Writer.write`, `writeAll`, `writeLn`, `writeBool`, `writeInt64`,
  `writeUint64`, and `writeFloat64` expose the corresponding writer operations
  directly on the buffered writer.
- `Writer.close() !void` is a throwing `destr` method that flushes and frees the buffer.
- `bufferedWrite(bw Writer*, bytes str) !u64` is the internal adapter callback, buffering small writes and directly handling large ones.

## Reader API

- `pub readerBuffered(a alc.Allocator, r reader.Reader) !$Reader` creates a buffered reader.
- `Reader.fillBuffer() !bool` refills the buffer and reports whether data was obtained.
- `Reader.filledCount()`, `isEof()`, `setFilled(value)`, and `markEof()` expose
  the buffered state used by reader adapters.
- `Reader.reader() reader.Reader` returns a generic reader view; the buffered reader must outlive it.
- `Reader.readLn(a alc.Allocator) !$str` reads through the next newline and returns an owned string without `\n`. It returns the final unterminated line at EOF.
- `Reader.close() void` is a `destr` method that frees the internal buffer.
- `bufferedRead(br Reader*, buff u8[], nBytes u64) !u64` is the internal reader callback.
- `resizeLineBuffer(a alc.Allocator, old u8*, newCapacity u64) !$u8*` is the checked internal growth helper used by `readLn`.
