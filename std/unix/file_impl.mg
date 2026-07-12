mod file_impl_unix

use "../file.mg"      file
use "../allocator.mg" alc
use "../slices.mg"    slices
use "../strings.mg"   strings
use "../cast.mg"      cast
use "../errors.mg"    errors
use "../writer.mg"    writer
use "../reader.mg"    reader
use "../file_op_mode.mg" fopm

ext ext_unix_open  open(path u8*, flags i32, mode i32) i32
ext ext_unix_close close(fd i32) i32
ext ext_unix_write write(fd i32, buf ptr, count u64) i64
ext ext_unix_read  read(fd i32, buf ptr, count u64) i64
ext ext_unix_lseek lseek(fd i32, offset i64, whence i32) i64

gl_writeOnce_written i64
gl_readOnce_read i64

# Casts a pointer to a file descriptor integer.
# O(1).
ptoi32(x ptr) i32:
    ret cast.i64to32(cast.utoi(cast.ptou(x)))
..

# Casts file descriptor integer to a pointer.
# O(1).
i32top(x i32) ptr:
    ret cast.utop(cast.itou(cast.i32to64(x)))
..

# Writes up to amount bytes once using unix write.
# O(1) per call.
writeOnce(fd i32, next ptr, amount u64) !u64:
   gl_writeOnce_written = ext_unix_write(fd, next, amount)
   
   if gl_writeOnce_written < 0:
      throw errors.failure("write failed")
   ..
   ret cast.itou(gl_writeOnce_written)
..

# Writes a string to a unix file handle.
# O(N) for byte count.
# @param handle file handle
# @param bytes string to write
# @returns bytes written
pub write(handle ptr, bytes str) !u64:
    fd i32 = ptoi32(handle)
    bound u64 = strings.countBytes(bytes)

    if bound == 0:
        ret 0
    ..

    p ptr = strings.toPtr(bytes)
    total u64 = 0

    while total < bound:
        toWrite u64 = bound - total

        next ptr = cast.utop(cast.ptou(p) + total)
        written u64 = try writeOnce(fd, next, toWrite)

        total = total + written

        if written == 0:
            break
        ..
    ..
    ret total
..

# Reads up to amount bytes once using unix read.
# O(1) per call.
readOnce(fd i32, next ptr, amount u64) !u64:
   gl_readOnce_read = ext_unix_read(fd, next, amount)

   if gl_readOnce_read < 0:
      throw errors.failure("read failed")
   ..

   ret cast.itou(gl_readOnce_read)
..

# Reads into a buffer from a unix file handle.
# O(N) for byte count.
# @param handle file handle
# @param buff destination buffer
# @param n max bytes to read
# @returns bytes read
pub read(handle ptr, buff u8[], n u64) !u64:
   if slices.count(buff) < n:
      throw errors.invalidArgument("read would overflow buffer")
   ..
   fd i32 = ptoi32(handle)
   bound u64 = n
   p ptr = slices.toPtr(buff)

   if n == 0:
      ret 0
   ..

   total u64 = 0

   while total < bound:
      toRead u64 = bound - total

      next ptr = cast.utop(cast.ptou(p) + total)
      read u64 = try readOnce(fd, next, toRead)

      total = total + read

      if read == 0:
         break
      ..
   ..
   ret total
..

# Returns a writer for standard output.
# O(1).
pub stdout() writer.Writer:
    ret writer.new(cast.utop(1), write)
..

# Returns a writer for standard error.
# O(1).
pub stderr() writer.Writer:
    ret writer.new(cast.utop(2), write)
..

# Returns a reader for standard input.
# O(1).
pub stdin() reader.Reader:
    ret reader.new(cast.utop(0), read)
..

# Closes a unix file handle.
# O(1).
pub closeFile(handle ptr) !void:
   fd i32 = ptoi32(handle)
   if ext_unix_close(fd) != 0:
      throw errors.failure("close failed")
   ..
..

# Opens a file using unix open.
# O(1) aside from path conversion and syscalls.
# @param a allocator to use
# @param path UTF-8 path
# @param openMode desired open mode
# @returns handle to the opened file
pub openFile(a alc.Allocator, path str, openMode fopm.OpenMode) !$ptr:
    O_RDONLY i32 = 0
    O_WRONLY i32 = 1
    O_RDWR   i32 = 2
    O_CREAT  i32 = 64
    O_TRUNC  i32 = 512
    O_APPEND i32 = 1024

    flags i32 = 0
    mode i32 = 438  # 0666 in octal

    if openMode.r && openMode.w == false:
        flags = O_RDONLY
    elif openMode.w && openMode.r == false:
        flags = O_WRONLY
        if openMode.a == false:
            flags = flags | O_CREAT | O_TRUNC
        else:
            flags = flags | O_CREAT | O_APPEND
        ..
    elif openMode.r && openMode.w:
        flags = O_RDWR
        if openMode.a == false && openMode.w:
            flags = flags | O_CREAT | O_TRUNC
        elif openMode.a:
            flags = flags | O_CREAT | O_APPEND
        ..
    else:
        throw errors.invalidArgument("invalid open mode")
    ..

    path_cstr u8* = try strings.toCstr(a, path)
    defer a.free(path_cstr)

    fd i32 = ext_unix_open(path_cstr, flags, mode)

    if fd < 0:
        throw errors.failure("open failure")
    ..
    ret i32top(fd)
..

# Moves a unix file descriptor's file offset.
# O(1).
# @param handle file handle
# @param offset signed offset relative to whence
# @param whence 0 for start, 1 for current position, 2 for end
# @returns the resulting absolute file position
pub seek(handle ptr, offset i64, whence u8) !u64:
    SEEK_SET i32 = 0
    SEEK_CUR i32 = 1
    SEEK_END i32 = 2

    origin i32 = 0
    if whence == 0:
        origin = SEEK_SET
    elif whence == 1:
        origin = SEEK_CUR
    elif whence == 2:
        origin = SEEK_END
    else:
        throw errors.invalidArgument("invalid whence")
    ..

    position i64 = ext_unix_lseek(ptoi32(handle), offset, origin)
    if position < 0:
        throw errors.failure("seek failed")
    ..

    ret cast.itou(position)
..
