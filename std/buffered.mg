mod buffered

use "allocator.mg" alc
use "writer.mg"    writer
use "reader.mg"    reader
use "errors.mg"    errors
use "slices.mg"    slices
use "strings.mg"   strings
use "cast.mg"      cast
use "memory.mg"    mem

# Buffered writer that accumulates writes before flushing.
# Reduces syscall overhead for many small writes.
# O(1) for most operations until buffer fills.
Writer(
    underlying writer.Writer
    buffer ptr
    bufferSize u64
    position u64
    allocator alc.Allocator
)

# Creates a buffered writer with default buffer size.
# Warning: caller must call close() or flush() to ensure all data is written.
# O(1) aside from allocation.
# @param a allocator for buffer
# @param w underlying writer
# @returns buffered writer
pub writerBuffered(a alc.Allocator, w writer.Writer) !$Writer:
    DEFAULT_BUFFER_SIZE u64 = 8192

    bw Writer
    bw.underlying = w
    bw.buffer = try a.alloc(DEFAULT_BUFFER_SIZE)
    bw.bufferSize = DEFAULT_BUFFER_SIZE
    bw.position = 0
    bw.allocator = a
    ret bw
..

# Flushes buffered data to underlying writer.
# Handles partial writes, errors, and maintains buffer consistency.
# O(N) for bytes in buffer.
# @returns total bytes written
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
# O(N) for byte count, amortized O(1) for small writes.
bufferedWrite(bw Writer*, bytes str) !u64:
    bytesLen u64 = strings.countBytes(bytes)
    
    if bytesLen == 0:
        ret 0
    ..
    
    # If write is larger than buffer, flush and write directly
    if bytesLen >= bw.bufferSize:
        try bw.flush()
        ret try bw.underlying.write(bytes)
    ..
    
    # If write doesn't fit in remaining buffer space, flush first
    available u64 = bw.bufferSize - bw.position
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
# O(1).
# @returns writer interface
Writer.writer() writer.Writer:
    ret writer.new(this, bufferedWrite)
..

# Closes the buffered writer, flushing any remaining data.
# O(N) for remaining buffered bytes.
Writer.close() !void:
    try this.flush()
    this.allocator.free(this.buffer)
    this.buffer = cast.utop(0)
    this.bufferSize = 0
    this.position = 0
..

# Buffered reader that reads in chunks and serves from buffer.
# Reduces syscall overhead for many small reads.
# O(1) for most operations when reading from buffer.
Reader(
    underlying reader.Reader
    buffer u8*
    bufferSize u64
    position u64   # Current read position in buffer
    filled u64     # How much of buffer contains valid data
    allocator alc.Allocator
    eof bool       # Hit end of underlying stream
)

# Creates a buffered reader with default buffer size.
# O(1) aside from allocation.
# @param a allocator for buffer
# @param r underlying reader
# @returns buffered reader
pub readerBuffered(a alc.Allocator, r reader.Reader) !$Reader:
    DEFAULT_BUFFER_SIZE u64 = 8192

    br Reader
    br.underlying = r
    br.buffer = try a.alloc(DEFAULT_BUFFER_SIZE)
    br.bufferSize = DEFAULT_BUFFER_SIZE
    br.position = 0
    br.filled = 0
    br.allocator = a
    br.eof = false
    ret br
..

# Fills the internal buffer from underlying reader.
# O(N) for buffer size.
Reader.fillBuffer() !bool:
    if this.eof:
        ret false
    ..
    
    # If there's unread data, move it to front of buffer
    if this.position < this.filled:
        unread u64 = this.filled - this.position
        srcPtr ptr = cast.utop(cast.ptou(this.buffer) + this.position)
        mem.move(srcPtr, this.buffer, unread)
        this.filled = unread
        this.position = 0
    else:
        this.filled = 0
        this.position = 0
    ..
    
    # Try to fill rest of buffer
    toRead u64 = this.bufferSize - this.filled
    readPtr ptr = cast.utop(cast.ptou(this.buffer) + this.filled)
    buffSlice u8[] = slices.fromPtr(readPtr, toRead)
    
    readCount u64 = try this.underlying.readToBuff(buffSlice, toRead)
    if readCount > toRead:
        throw errors.failure("buffered reader returned too many bytes")
    ..
    this.filled = this.filled + readCount
    
    if readCount == 0:
        this.eof = true
    ..
    ret readCount > 0
..

# Internal read implementation for Reader.
# O(N) for requested bytes, amortized O(1) when reading from buffer.
bufferedRead(br Reader*, buff u8[], nBytes u64) !u64:
    if nBytes == 0:
        ret 0
    ..
    
    totalRead u64 = 0
    dstPtr ptr = cast.utop(0)

    while totalRead < nBytes:
        # Serve from buffer if available
        available u64 = br.filled - br.position
        
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
        if br.eof:
            break
        ..
        
        # For large reads, bypass buffer and read directly
        remaining u64 = nBytes - totalRead
        if remaining >= br.bufferSize:
            dstPtr = cast.utop(cast.ptou(slices.toPtr(buff)) + totalRead)
            directBuff u8[] = slices.fromPtr(dstPtr, remaining)
            directRead u64 = try br.underlying.readToBuff(directBuff, remaining)
            if directRead > remaining:
                throw errors.failure("buffered reader returned too many bytes")
            ..
            totalRead = totalRead + directRead
            
            if directRead == 0:
                br.eof = true
            ..
            break
        ..
        try br.fillBuffer()
    ..
    ret totalRead
..

# Returns a Reader interface for this buffered reader.
# O(1).
# @returns reader interface
Reader.reader() reader.Reader:
    ret reader.new(this, bufferedRead)
..

resizeLineBuffer(a alc.Allocator, old u8*, newCapacity u64) !$u8*:
    resized u8*, resizeErr error = a.realloc(old, newCapacity)
    if errors.code(resizeErr) != 0:
        a.free(old)
        throw resizeErr
    ..
    ret resized
..

# Reads a line (up to \n) from the buffered reader.
# Returns string without the newline character.
# O(N) for line length.
# @param a allocator for result string
# @returns line as string
Reader.readLn(a alc.Allocator) !$str:
    # Initial capacity for line buffer
    capacity u64 = 128
    lineBuffer u8* = try a.alloc(capacity)
    lineLen u64 = 0
    dstPtr ptr = cast.utop(0)
    newCapacity u64 = 0
    
    while true:
        # Check if we need more buffer space
        if lineLen >= capacity:
            newCapacity = capacity * 2
            lineBuffer = try resizeLineBuffer(a, lineBuffer, newCapacity)
            capacity = newCapacity
        ..
        
        # Look for newline in current buffer
        available u64 = this.filled - this.position
        
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
        if this.eof:
            # Return what we have (even if no newline)
            if lineLen > 0:
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
# O(1).
Reader.close() void:
    this.allocator.free(this.buffer)
    this.buffer = cast.utop(0)
    this.bufferSize = 0
    this.position = 0
    this.filled = 0
..
