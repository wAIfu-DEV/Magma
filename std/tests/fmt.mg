mod main

use "std:allocator" allocator
use "std:errors" errors
use "std:fake_alloc" fake_alloc
use "std:fmt" fmt
use "std:heap" heap
use "std:strings" strings
use "std:writer" writer

Capture impl writer.Writer(
    buffer u8*
    count u64
)

Capture.write(bytes str) !u64:
    count := bytes.countBytes()
    # SAFETY: impl is a Capture and main provides a 128-byte buffer, larger
    # than every write made by this test.
    unsafe:
        i u64 = 0
        loop i < count:
            this.buffer[this.count + i] = strings.byteAt(bytes, i)
            i = i + 1
        ..
        this.count = this.count + count
    ..
    ret count
..

pub main() !void:
    a allocator.Allocator = heap.allocator()

    rendered $str, renderErr error = fmt.str(a, "Value: ").uint(5).str(", signed: ").int(-2).str(", active: ").bool(true).str(", ratio: ").float(1.5, 1).toStr(a)
    if renderErr.nok():
        throw renderErr
    ..
    defer rendered.free(a)
    if strings.compare(rendered, "Value: 5, signed: -2, active: true, ratio: 1.5") == false:
        throw errors.failure("formatted string changed")
    ..

    captureBytes := try strings.alloc(a, 128)
    defer captureBytes.free(a)
    capture := Capture(buffer=strings.toPtr(captureBytes), count=0)
    output := capture.proto[writer.Writer]()
    written := try fmt.str(a, "one").str(" two").str(" three").str(" four").str(" five").str(" six").str(" seven").str(" eight").str(" nine").writeTo(output)
    captured := strings.fromPtrNoCopy(capture.buffer, capture.count)
    if written != 44 || strings.compare(captured, "one two three four five six seven eight nine") == false:
        throw errors.failure("format growth or writer output changed")
    ..

    failing allocator.Allocator = fake_alloc.allocator()
    failedFormat := fmt.str(failing, "not written").uint(7)
    ignored u64, formatErr error = failedFormat.writeTo(output)
    if formatErr.ok() || capture.count != 44:
        throw errors.failure("sticky format allocation error changed")
    ..
..
