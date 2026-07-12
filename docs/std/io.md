# `std/io`

Standard process-stream adapters.

- `pub stdout(a alc.Allocator) !$buffered.Writer` creates an owned buffered standard-output writer; close or flush it.
- `pub stdoutUnbuffered() writer.Writer` returns an unbuffered standard-output writer.
- `pub stderr(a alc.Allocator) !$buffered.Writer` creates an owned buffered standard-error writer; close or flush it.
- `pub stderrUnbuffered() writer.Writer` returns an unbuffered standard-error writer.
- `pub stdin(a alc.Allocator) !$buffered.Reader` creates an owned buffered standard-input reader; close it.
- `pub stdinUnbuffered() reader.Reader` returns an unbuffered standard-input reader.

The buffered constructors can fail while allocating their buffers.
