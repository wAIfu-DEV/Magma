# `std/file`

## Example

```magma
output := try file.open("report.txt", file.mode().write())
try output.writer().writeAll("ready\n")
try output.close()
```

Cross-platform file handles and adapters. Platform-specific system calls are selected internally.

## Type

`File(handle ptr, openMode fopm.OpenMode, open bool)` stores the native handle, granted mode flags, and open state.

## API

- `pub open(path str, openMode fopm.OpenMode) !$File` opens `path`. The caller owns the wrapper and must call `close()`.
- `pub mode() fopm.OpenMode` returns an empty mode value for fluent configuration, such as `file.mode().read().write()`.
- `File.close() !void` is a throwing `destr` method that closes an open handle; calling it again is harmless.
- `File.writer() !w.Writer` creates a borrowed writer adapter and requires write mode.
- `File.reader() !r.Reader` creates a borrowed reader adapter and requires read mode.
- `File.seek(offset i64, whence u8) !u64` moves the file position and returns the resulting absolute position. `whence` uses 0 for start, 1 for current, and 2 for end.
- `File.count() !u64` obtains the file size while restoring the previous position.
- `File.readAsync(offset u64, count u64) !$future.Future[str]` starts a
  positional, abortable read without changing the cursor on Unix.
- `File.readAsyncAbort(offset u64, count u64, signal abort.Signal)` also
  observes a caller-owned signal.

The file must remain open through `await()`. Reads are performed in 256 KiB
chunks so portable worker-backed implementations can observe abort requests.
The synchronous Windows positional fallback must not be mixed concurrently
with cursor-based operations on the same file; a future overlapped backend can
replace it without changing this API.

`write(f File*, bytes str) !u64` and `read(f File*, buff u8[], n u64) !u64` are internal adapter callbacks. Operations on a closed file produce `invalidArgument`.
