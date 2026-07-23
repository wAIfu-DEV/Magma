mod io
# Standard input, output, and error stream access.

use "std:writer"    writer
use "std:reader"    reader
use "std:buffered"  buffered
use "std:allocator" alc

@platform("windows")
use "std:win/file_impl" impl_file

@platform("linux", "android", "ios", "darwin", "freebsd", "netbsd", "openbsd")
use "std:unix/file_impl" impl_file

# Returns a buffered writer for standard output.
# @complexity O(1).
# @ownership Close the returned writer to flush and release its buffer.
# @example
#   output := try io.stdout(a)
pub stdout(a alc.Allocator) !$buffered.Writer:
    ret try buffered.writerBuffered(a, impl_file.stdout())
..

# Returns a buffered writer for standard output.
# @complexity O(1).
# @ownership The returned pointer is global and must not be freed.
pub stdoutConst() writer.ConstWriter*:
    ret impl_file.stdoutConst()
..

# Returns a writer for standard output.
# @complexity O(1).
# @ownership The returned writer borrows the process standard-output handle.
pub stdoutUnbuffered() writer.Writer:
    ret impl_file.stdout()
..

# Writes bytes to standard output using the globally constant writer interface.
# @complexity O(N)
# @returns number of bytes written
# @example
#   try io.print("working")
pub print(bytes str) !u64:
    out := impl_file.stdoutConst()
    ret try out.writeAll(bytes)
..

# Writes bytes followed by a newline using the globally constant writer interface.
# @complexity O(N)
# @returns number of bytes written, including the newline
# @example
#   try io.printLn("complete")
pub printLn(bytes str) !u64:
    out := impl_file.stdoutConst()
    ret try out.writeLn(bytes)
..

# Returns a buffered writer for standard error.
# @complexity O(1).
# @ownership Close the returned writer to flush and release its buffer.
pub stderr(a alc.Allocator) !$buffered.Writer:
    ret try buffered.writerBuffered(a, impl_file.stderr())
..

# Returns a writer for standard error.
# @complexity O(1).
# @ownership The returned writer borrows the process standard-error handle.
pub stderrUnbuffered() writer.Writer:
    ret impl_file.stderr()
..

# Returns a buffered reader for standard input.
# @complexity O(1).
# @ownership Close the returned reader to release its buffer.
pub stdin(a alc.Allocator) !$buffered.Reader:
    ret try buffered.readerBuffered(a, impl_file.stdin())
..

# Returns a reader for standard input.
# @complexity O(1).
# @ownership The returned reader borrows the process standard-input handle.
pub stdinUnbuffered() reader.Reader:
    ret impl_file.stdin()
..
