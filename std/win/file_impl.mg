mod file_impl_win

use "../utf8.mg"      utf8
use "../allocator.mg" alc
use "../slices.mg"    slices
use "../strings.mg"   strings
use "../cast.mg"      cast
use "../errors.mg"    errors
use "../writer.mg"    writer
use "../reader.mg"    reader
use "../file_op_mode.mg" fopm

ext ext_win32_CreateFileW      CreateFileW(pathUtf16 i16*, accessMode u32, _arg0 i32, _arg1 ptr, createMode i32, _arg2 i32, _arg3 ptr) ptr
ext ext_win32_CloseHandle      CloseHandle(handle ptr) i32
ext ext_win32_WriteFile        WriteFile(handle ptr, arg0 ptr, arg1 u32, arg2 ptr, arg3 ptr) u32
ext ext_win32_ReadFile         ReadFile(handle ptr, arg0 ptr, arg1 u32, arg2 ptr, arg3 ptr) u32
ext ext_win32_GetStdHandle     GetStdHandle(handleNum i32) ptr
ext ext_win32_SetFilePointerEx SetFilePointerEx(handle ptr, distance i64, newPosition i64*, moveMethod u32) i32
ext ext_win32_GetLastError     GetLastError() u32

# Magma globals are thread-local by default. These syscall output slots avoid
# repeated stack allocation without sharing state between threads.
gl_writeOnce_written u32
gl_readOnce_read u32

# Writes up to amount bytes once using Win32 WriteFile.
# O(1) per call.
writeOnce(handle ptr, next ptr, amount u32) !u64:
   # HACK: using global var for out ptr
   # in order to minimize stack allocations, allows extreme inlining
   # using a stack allocated var forces LLVM to generate it at call site too since
   # call to external function requires valid state without assumptions,
   # leading to guaranteed alloca instruction for each write call.
   ok u32 = ext_win32_WriteFile(handle, next, amount, addrof gl_writeOnce_written, none)
   
   if ok == 0:
      throw errors.native(ext_win32_GetLastError(), "WriteFile failed")
   ..

   ret cast.u32to64(gl_writeOnce_written)
..

# Writes a string to a Win32 file handle.
# O(N) for byte count.
# @param handle file handle
# @param bytes string to write
# @returns bytes written
pub write(handle ptr, bytes str) !u64:
   bound u64 = strings.countBytes(bytes)

   if bound == 0:
      ret 0
   ..

   # happy path (short string)
   # should help optimize if size is known at comptime
   if bound <= 0xFFFFFFFF:
      ret try writeOnce(handle, strings.toPtr(bytes), cast.u64to32(bound))
   ..

   p ptr = strings.toPtr(bytes)
   total u64 = 0

   while total < bound:
      toWrite u32 = 0
      if (bound - total) > 0xFFFFFFFF:
         toWrite = 0xFFFFFFFF
      else:
         toWrite = cast.u64to32(bound - total)
      ..

      if toWrite == 0:
         break
      ..

      next ptr = cast.utop(cast.ptou(p) + total)
      written u64 = try writeOnce(handle, next, toWrite)

      total = total + written
      # Note: might need EOF flag reset

      if written < cast.u32to64(toWrite):
         break
      ..
   ..
   ret total
..

# Reads up to amount bytes once using Win32 ReadFile.
# O(1) per call.
readOnce(handle ptr, next ptr, amount u32) !u64:

   # HACK: see writeOnce
   ok u32 = ext_win32_ReadFile(handle, next, amount, addrof gl_readOnce_read, none)

   if ok == 0:
      throw errors.native(ext_win32_GetLastError(), "ReadFile failed")
   ..

   # Note: if read == 0 should set EOF flag
   # Future me: what the fuck are you talking about
   ret cast.u32to64(gl_readOnce_read)
..

# Reads into a buffer from a Win32 file handle.
# O(N) for byte count.
# @param handle file handle
# @param buff destination buffer
# @param n max bytes to read
# @returns bytes read
pub read(handle ptr, buff u8[], n u64) !u64:
   if slices.count(buff) < n:
      throw errors.invalidArgument("read would overflow buffer")
   ..
   if n == 0:
      ret 0
   ..
   # happy path (short string)
   # should help optimize if size is known at comptime
   if n <= 0xFFFFFFFF:
      ret try readOnce(handle, slices.toPtr(buff), cast.u64to32(n))
   ..

   bound u64 = n
   p ptr = slices.toPtr(buff)

   total u64 = 0

   while total < bound:
      toRead u32 = 0
      if (bound - total) > 0xFFFFFFFF:
         toRead = 0xFFFFFFFF
      else:
         toRead = cast.u64to32(bound - total)
      ..

      if toRead == 0:
         break
      ..

      next ptr = cast.utop(cast.ptou(p) + total)
      bytesRead u64 = try readOnce(handle, next, toRead)

      total = total + bytesRead

      if bytesRead < cast.u32to64(toRead):
         break
      ..
   ..
   ret total
..

# Returns a writer for the Win32 standard output handle.
# O(1).
pub stdout() writer.Writer:
   ret writer.new(ext_win32_GetStdHandle(-11), write)
..

# Returns a writer for the Win32 standard error handle.
# O(1).
pub stderr() writer.Writer:
   ret writer.new(ext_win32_GetStdHandle(-12), write)
..

# Returns a reader for the Win32 standard input handle.
# O(1).
pub stdin() reader.Reader:
   ret reader.new(ext_win32_GetStdHandle(-10), read)
..

# Closes a Win32 file handle.
# O(1).
pub closeFile(handle ptr) !void:
   if ext_win32_CloseHandle(handle) == 0:
      throw errors.native(ext_win32_GetLastError(), "CloseHandle failed")
   ..
..

# Opens a file using Win32 CreateFileW.
# O(1) aside from path conversion and syscalls.
# @param a allocator to use
# @param path UTF-8 path
# @param openMode desired open mode
# @returns handle to the opened file
pub openFile(a alc.Allocator, path str, openMode fopm.OpenMode) !$ptr:
   READ  u32 = 0x80000000
   WRITE u32 = 0x40000000
   APPEND u32 = 4

   OPEN_EXISTING i32 = 3
   CREATE_ALWAYS i32 = 2
   OPEN_ALWAYS i32 = 4

   access_mode u32
   open_mode i32

   if openMode.a:
      access_mode = APPEND
      if openMode.r:
         access_mode = access_mode | READ
      ..
      open_mode = OPEN_ALWAYS
   elif openMode.r && openMode.w:
      access_mode = READ | WRITE
      open_mode = CREATE_ALWAYS
   elif openMode.r:
      access_mode = READ
      open_mode = OPEN_EXISTING
   elif openMode.w:
      access_mode = WRITE
      open_mode = CREATE_ALWAYS
   else:
      throw errors.invalidArgument("invalid open mode")
   ..

   path_u16 u16[] = try utf8.utf8To16NT(a, path)
   path_ptr u16* =  slices.toPtr(path_u16)

   defer a.free(path_ptr) # frees created utf16 string

   handle ptr = ext_win32_CreateFileW(path_ptr, access_mode, 0, none, open_mode, 0, none)

   # invalid handle
   if cast.ptou(handle) == cast.itou(-1):
      if openMode.a:
         # create file if append mode
         handle = ext_win32_CreateFileW(path_ptr, access_mode, 0, none, 2, 0, none)
      ..

      if cast.ptou(handle) == cast.itou(-1):
          throw errors.native(ext_win32_GetLastError(), "CreateFileW failed")
      ..
   ..
   ret handle
..

pub seek(handle ptr, offset i64, whence u8) !u64:
   # Convert whence to Windows constants
   FILE_BEGIN   u32 = 0
   FILE_CURRENT u32 = 1
   FILE_END     u32 = 2
    
   moveMethod u32 = 0
   if whence == 0:
      moveMethod = FILE_BEGIN
   elif whence == 1:
      moveMethod = FILE_CURRENT
   elif whence == 2:
      moveMethod = FILE_END
   else:
      throw errors.invalidArgument("invalid whence")
   ..
    
   newPos i64 = 0
   if ext_win32_SetFilePointerEx(handle, offset, addrof newPos, moveMethod) == 0:
      throw errors.native(ext_win32_GetLastError(), "SetFilePointerEx failed")
   ..
   if newPos < 0:
      throw errors.failure("seek returned a negative position")
   ..
   ret cast.itou(newPos)
..
