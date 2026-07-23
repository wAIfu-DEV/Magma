mod buffered
# Buffered reader and writer adapters that reduce underlying I/O operations.

use "std:allocator" alc
use "std:writer"    writer
use "std:reader"    reader
use "std:errors"    errors
use "std:slices"    slices
use "std:strings"   strings
use "std:cast"      cast
use "std:memory"    mem
use "std:footgun"   footgun

const DEFAULT_BUFFER_SIZE u64 = 8192
const EOF_MASK u64 = 0x8000000000000000
const FILLED_MASK u64 = 0x7FFFFFFFFFFFFFFF

# Buffered writer that accumulates writes before flushing.
# Reduces syscall overhead for many small writes.
# @complexity O(1) for most operations until buffer fills.
pub Writer(
    underlying writer.Writer
    buffer ptr
    position u64
    allocator alc.Allocator
)

# Creates a buffered writer with default buffer size.
# @warning caller must call close() or flush() to ensure all data is written.
# @complexity O(1) aside from allocation.
# @param a allocator for buffer
# @param w underlying writer
# @returns buffered writer
# @ownership The returned writer owns its buffer and must be closed.
# @example
#   bufferedWriter := try buffered.writerBuffered(a, output)
pub writerBuffered(a alc.Allocator, w writer.Writer) !$Writer:
    ret Writer(
        underlying=w,
        buffer=try a.alloc(DEFAULT_BUFFER_SIZE),
        position=0,
        allocator=a,
    )
..

# Flushes buffered data to underlying writer.
# Handles partial writes, errors, and maintains buffer consistency.
# @complexity O(N) for bytes in buffer.
# @returns total bytes written
# @example
#   try bufferedWriter.flush()
Writer.flush() !u64:
    if this.position == 0:
        ret 0
    ..
    
    totalWritten u64 = 0
    remaining u64 = this.position
    writePtr ptr = this.buffer
    
    while remaining > 0:
        toWrite str = strings.fromPtrNoCopy(writePtr, remaining)
        written u64 = try this.underlying.write(toWrite)
        if written > remaining:
            this.position = remaining
            throw errors.failure("flush failed: writer returned too many bytes")
        ..
        
        totalWritten = totalWritten + written
        remaining = remaining - written
        
        if remaining == 0:
            this.position = 0
            ret totalWritten
        ..
        
        if written > 0:
            unwrittenPtr ptr = cast.utop(cast.ptou(this.buffer) + written)
            mem.move(unwrittenPtr, this.buffer, remaining)
            this.position = remaining
            writePtr = this.buffer
        else:
            this.position = remaining
            throw errors.failure("flush failed: underlying writer wrote 0 bytes")
        ..
    ..
    
    # Should never reach here
    this.position = 0
    ret totalWritten
..

# Internal write implementation for Writer.
# @complexity O(N) for byte count, amortized O(1) for small writes.
bufferedWrite(bw Writer*, bytes str) !u64:
    bytesLen u64 = strings.countBytes(bytes)
    
    if bytesLen == 0:
        ret 0
    ..
    
    # If write is larger than buffer, flush and write directly
    if bytesLen >= DEFAULT_BUFFER_SIZE:
        try bw.flush()
        ret try bw.underlying.write(bytes)
    ..
    
    # If write doesn't fit in remaining buffer space, flush first
    available u64 = DEFAULT_BUFFER_SIZE - bw.position
    if bytesLen > available:
        try bw.flush()
    ..
    
    # Copy to buffer
    srcPtr ptr = strings.toPtr(bytes)
    dstPtr ptr = cast.utop(cast.ptou(bw.buffer) + bw.position)
    mem.copy(srcPtr, dstPtr, bytesLen)
    bw.position = bw.position + bytesLen
    
    ret bytesLen
..

# Returns a Writer interface for this buffered writer.
# @complexity O(1).
# @returns writer interface
# @ownership The interface borrows this Writer, which must remain alive and unmoved.
# @example
#   output := bufferedWriter.writer()
Writer.writer() writer.Writer:
    ret writer.new(this, bufferedWrite)
..

# Closes the buffered writer, flushing any remaining data.
# @complexity O(N) for remaining buffered bytes.
# @warning If close throws, the buffer remains allocated so the caller can retry.
# @example
#   try bufferedWriter.close()
destr Writer.close() !void:
    try this.flush()
    this.allocator.free(this.buffer)
    this.buffer = none
    this.position = 0
..

# Writes the provided bytes and returns the count written.
# @complexity O(N) for byte count.
# @param bytes string to write
# @returns number of bytes written
# @note This convenience method writes directly to the underlying writer; use writer() for buffering.
# @example
#   written := try bufferedWriter.write("data")
Writer.write(bytes str) !u64:
    ret try this.underlying.write(bytes)
..

# Writes the complete byte string or returns an error if the adapter makes no
# progress or reports an invalid count.
# @complexity O(N) for byte count
# @note This convenience method writes directly to the underlying writer; use writer() for buffering.
# @example
#   try bufferedWriter.writeAll("complete payload")
Writer.writeAll(bytes str) !u64:
    ret try this.underlying.writeAll(bytes)
..

# Writes the provided bytes followed by a newline.
# @complexity O(N) for byte count.
# @param bytes string to write
# @returns number of bytes written
# @example
#   try bufferedWriter.writeLn("record")
Writer.writeLn(bytes str) !u64:
    ret try this.underlying.writeLn(bytes)
..

# Writes "true" or "false" based on the boolean value.
# @complexity O(1).
# @param b boolean value
# @returns number of bytes written
# @example
#   try bufferedWriter.writeBool(true)
Writer.writeBool(b bool) !u64:
    ret try this.underlying.writeBool(b)
..

# Writes a signed 64-bit integer in decimal form.
# @complexity O(1) bounded by integer width.
# @param num integer value
# @returns number of bytes written
# @example
#   try bufferedWriter.writeInt64(-42)
Writer.writeInt64(num i64) !u64:
    ret try this.underlying.writeInt64(num)
..

# Writes an unsigned 64-bit integer in decimal form.
# @complexity O(1) bounded by integer width.
# @param num integer value
# @returns number of bytes written
# @example
#   try bufferedWriter.writeUint64(42)
Writer.writeUint64(num u64) !u64:
    ret try this.underlying.writeUint64(num)
..


# Writes a floating point value with the provided precision.
# @complexity O(P) for precision digits.
# @param flt floating point value
# @param precision digits after decimal point
# @returns number of bytes written
# @example
#   try bufferedWriter.writeFloat64(3.14159, 2)
Writer.writeFloat64(flt f64, precision u64) !u64:
    ret try this.underlying.writeFloat64(flt, precision)
..

# Buffered reader that reads in chunks and serves from buffer.
# Reduces syscall overhead for many small reads.
# @complexity O(1) for most operations when reading from buffer.
pub Reader(
    underlying reader.Reader
    buffer u8*
    position u64   # Current read position in buffer
    filled u64     # How much of buffer contains valid data
    allocator alc.Allocator
)

Reader.filledCount() u64:
    ret this.filled & FILLED_MASK
..

Reader.isEof() bool:
    ret (this.filled & EOF_MASK) != 0
..

Reader.setFilled(value u64) void:
    this.filled = (this.filled & EOF_MASK) | value
..

Reader.markEof() void:
    this.filled = this.filled | EOF_MASK
..

# Creates a buffered reader with default buffer size.
# @complexity O(1) aside from allocation.
# @param a allocator for buffer
# @param r underlying reader
# @returns buffered reader
# @ownership The returned reader owns its buffer and must be closed.
# @example
#   bufferedReader := try buffered.readerBuffered(a, input)
pub readerBuffered(a alc.Allocator, r reader.Reader) !$Reader:
    ret Reader(
        underlying=r,
        buffer=try a.alloc(DEFAULT_BUFFER_SIZE),
        position=0,
        filled=0,
        allocator=a,
    )
..

# Fills the internal buffer from underlying reader.
# @complexity O(N) for buffer size.
Reader.fillBuffer() !bool:
    if this.isEof():
        ret false
    ..
    
    # If there's unread data, move it to front of buffer
    filled := this.filledCount()
    if this.position < filled:
        unread u64 = filled - this.position
        srcPtr ptr = cast.utop(cast.ptou(this.buffer) + this.position)
        mem.move(srcPtr, this.buffer, unread)
        this.setFilled(unread)
        this.position = 0
    else:
        this.setFilled(0)
        this.position = 0
    ..
    
    # Try to fill rest of buffer
    filled = this.filledCount()
    toRead u64 = DEFAULT_BUFFER_SIZE - filled
    readPtr ptr = cast.utop(cast.ptou(this.buffer) + filled)
    buffSlice u8[] = slices.fromPtr(readPtr, toRead)
    
    readCount u64 = try this.underlying.readToBuff(buffSlice, toRead)
    if readCount > toRead:
        throw errors.failure("buffered reader returned too many bytes")
    ..
    this.setFilled(filled + readCount)
    
    if readCount == 0:
        this.markEof()
    ..
    ret readCount > 0
..

# Internal read implementation for Reader.
# @complexity O(N) for requested bytes, amortized O(1) when reading from buffer.
bufferedRead(br Reader*, buff u8[], nBytes u64) !u64:
    if nBytes == 0:
        ret 0
    ..
    
    totalRead u64 = 0
    dstPtr ptr = none

    while totalRead < nBytes:
        # Serve from buffer if available
        available u64 = br.filledCount() - br.position
        
        if available > 0:
            toCopy u64 = nBytes - totalRead
            if toCopy > available:
                toCopy = available
            ..
            
            srcPtr ptr = cast.utop(cast.ptou(br.buffer) + br.position)
            dstPtr = cast.utop(cast.ptou(slices.toPtr(buff)) + totalRead)
            mem.copy(srcPtr, dstPtr, toCopy)
            
            br.position = br.position + toCopy
            totalRead = totalRead + toCopy
            continue
        ..
        
        # Buffer exhausted, need to refill
        if br.isEof():
            break
        ..
        
        # For large reads, bypass buffer and read directly
        remaining u64 = nBytes - totalRead
        if remaining >= DEFAULT_BUFFER_SIZE:
            dstPtr = cast.utop(cast.ptou(slices.toPtr(buff)) + totalRead)
            directBuff u8[] = slices.fromPtr(dstPtr, remaining)
            directRead u64 = try br.underlying.readToBuff(directBuff, remaining)
            if directRead > remaining:
                throw errors.failure("buffered reader returned too many bytes")
            ..
            totalRead = totalRead + directRead
            
            if directRead == 0:
                br.markEof()
            ..
            break
        ..
        try br.fillBuffer()
    ..
    ret totalRead
..

# Returns a Reader interface for this buffered reader.
# @complexity O(1).
# @returns reader interface
# @ownership The interface borrows this Reader, which must remain alive and unmoved.
# @example
#   input := bufferedReader.reader()
Reader.reader() reader.Reader:
    ret reader.new(this, bufferedRead)
..

resizeLineBuffer(a alc.Allocator, old u8*, newCapacity u64) !$u8*:
    if newCapacity == 0 - 1:
        a.free(old)
        throw errors.wouldOverflow("line buffer capacity overflow")
    ..
    resized u8*, resizeErr error = a.realloc(old, newCapacity + 1)
    if errors.code(resizeErr) != 0:
        a.free(old)
        throw resizeErr
    ..
    ret resized
..

# Reads a line (up to \n) from the buffered reader.
# Returns string without the newline character.
# @complexity O(N) for line length.
# @param a allocator for result string
# @returns line as string
# @ownership The caller owns the returned string and must free it with a.
# @throws endOfFile when no bytes remain; a final unterminated line is returned first
# @example
#   line := try bufferedReader.readLn(a)
Reader.readLn(a alc.Allocator) !$str:
    # Initial capacity for line buffer
    capacity u64 = 128
    line $str = try strings.alloc(a, capacity)

    # Compiler warning suppression
    defer footgun.drop[str](line)

    lineBuffer u8* = strings.toPtr(line)
    lineLen u64 = 0
    dstPtr ptr = none
    newCapacity u64 = 0
    
    while true:
        # Check if we need more buffer space
        if lineLen >= capacity:
            if capacity > (0 - 1) / 2:
                a.free(lineBuffer)
                throw errors.wouldOverflow("line buffer capacity overflow")
            ..
            newCapacity = capacity * 2
            lineBuffer = try resizeLineBuffer(a, lineBuffer, newCapacity)
            capacity = newCapacity
        ..
        
        # Look for newline in current buffer
        available u64 = this.filledCount() - this.position
        
        if available > 0:
            searchStart ptr = cast.utop(cast.ptou(this.buffer) + this.position)
            searchPtr u8* = searchStart
            
            i u64 = 0
            found bool = false
            foundPos u64 = 0
            nlLen u64 = 1
            nextPos u64 = 0
            
            while i < available:
                if searchPtr[i] == 10:  # '\n'
                    found = true
                    foundPos = i
                    break
                ..

                if searchPtr[i] == 13 && i+1 < available: # '\r'
                    nextPos = i + 1
                    if searchPtr[nextPos] == 10:  # '\n'
                        found = true
                        foundPos = i
                        nlLen = 2
                        break
                    ..
                ..

                i = i + 1
            ..
            
            if found:
                # Copy up to (not including) newline
                if foundPos > 0:
                    # Ensure capacity
                    if lineLen + foundPos > capacity:
                        newCapacity = lineLen + foundPos
                        lineBuffer = try resizeLineBuffer(a, lineBuffer, newCapacity)
                        capacity = newCapacity
                    ..
                    
                    dstPtr = cast.utop(cast.ptou(lineBuffer) + lineLen)
                    mem.copy(searchStart, dstPtr, foundPos)
                    lineLen = lineLen + foundPos
                ..
                
                # Skip past newline
                this.position = this.position + foundPos + nlLen
                lineBuffer[lineLen] = 0
                ret strings.fromPtrNoCopy(lineBuffer, lineLen)
            ..
            
            # No newline found, copy all available
            if lineLen + available > capacity:
                newCapacity = lineLen + available
                lineBuffer = try resizeLineBuffer(a, lineBuffer, newCapacity)
                capacity = newCapacity
            ..
            
            dstPtr = cast.utop(cast.ptou(lineBuffer) + lineLen)
            mem.copy(searchStart, dstPtr, available)
            lineLen = lineLen + available
            this.position = this.position + available
        ..
        
        # Refill buffer
        if this.isEof():
            # Return what we have (even if no newline)
            if lineLen > 0:
                lineBuffer[lineLen] = 0
                ret strings.fromPtrNoCopy(lineBuffer, lineLen)
            ..
            a.free(lineBuffer)
            throw errors.endOfFile("end of file")
        ..
        
        filled bool, fillErr error = this.fillBuffer()
        if errors.code(fillErr) != 0:
            a.free(lineBuffer)
            throw fillErr
        ..
    ..
    
    # Should never reach here
    a.free(lineBuffer)
    throw errors.failure("unexpected error in readLn")
..

# Closes the buffered reader and frees buffer.
# @complexity O(1).
# Releases the internal read buffer and invalidates interfaces borrowed from this reader.
# @complexity O(1)
# @example
#   bufferedReader.close()
destr Reader.close() void:
    this.allocator.free(this.buffer)
    this.buffer = none
    this.position = 0
    this.filled = 0
..
