mod io

use "strings.mg"   strings
use "writer.mg"    writer
use "reader.mg"    reader
use "buffered.mg"  buffered
use "allocator.mg" alc

@platform("windows")
use "win/file_impl.mg" impl_file

@platform("linux", "android", "ios", "darwin", "freebsd", "netbsd", "openbsd")
use "unix/file_impl.mg" impl_file

# Returns a buffered writer for standard output.
# O(1).
pub stdout(a alc.Allocator) !$buffered.Writer:
    ret try buffered.writerBuffered(a, impl_file.stdout())
..

# Returns a writer for standard output.
# O(1).
pub stdoutUnbuffered() writer.Writer:
    ret impl_file.stdout()
..

# Returns a buffered writer for standard error.
# O(1).
pub stderr(a alc.Allocator) !$buffered.Writer:
    ret try buffered.writerBuffered(a, impl_file.stderr())
..

# Returns a writer for standard error.
# O(1).
pub stderrUnbuffered() writer.Writer:
    ret impl_file.stderr()
..

# Returns a buffered reader for standard input.
# O(1).
pub stdin(a alc.Allocator) !$buffered.Reader:
    ret try buffered.readerBuffered(a, impl_file.stdin())
..

# Returns a reader for standard input.
# O(1).
pub stdinUnbuffered() reader.Reader:
    ret impl_file.stdin()
..