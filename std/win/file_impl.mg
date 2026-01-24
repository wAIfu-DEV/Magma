mod file_impl_win

use "../file.mg"      file
use "../utf8.mg"      utf8
use "../allocator.mg" alc
use "../slices.mg"    slices
use "../strings.mg"   strings
use "../cast.mg"      cast
use "../errors.mg"    errors
use "../writer.mg"    writer
use "../reader.mg"    reader

ext ext_win32_CreateFileW  CreateFileW(pathUtf16 i16*, accessMode u32, _arg0 i32, _arg1 ptr, createMode i32, _arg2 i32, _arg3 ptr) ptr
ext ext_win32_CloseHandle  CloseHandle(handle ptr) i32

ext ext_win32_WriteFile    WriteFile(handle ptr, arg0 ptr, arg1 u32, arg2 ptr, arg3 ptr) u32
llvm "declare i32 @WriteFile(ptr readonly nocapture, ptr, i32, ptr, ptr)\n"

ext ext_win32_ReadFile     ReadFile(handle ptr, arg0 ptr, arg1 u32, arg2 ptr, arg3 ptr) u32

ext ext_win32_GetStdHandle GetStdHandle(handleNum i32) ptr
llvm "declare nonnull ptr @GetStdHandle(i32) readnone\n"

gl_writeOnce_written u32
gl_readOnce_read u32

gl_handles_cached bool
gl_stdout_handle ptr
gl_stderr_handle ptr
gl_stdin_handle ptr

writeOnce(handle ptr, next ptr, amount u32) !u64:
   # HACK: using global var for out ptr
   # in order to minimize stack allocations, allows extreme inlining
   # using a stack allocated var forces LLVM to generate it at call site too since
   # call to external function requires valid state without assumptions,
   # leading to guaranteed alloca instruction for each write call.
   ok u32 = ext_win32_WriteFile(handle, next, amount, addrof gl_writeOnce_written, cast.utop(0))
   
   if ok == 0:
      # TODO: throw
      ret 0
   ..

   ret cast.u32to64(gl_writeOnce_written)
..

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

readOnce(handle ptr, next ptr, amount u32) !u64:

   # HACK: see writeOnce
   ok u32 = ext_win32_ReadFile(handle, next, amount, addrof gl_readOnce_read, cast.utop(0))

   if ok == 0:
      # TODO: throw
      ret 0
   ..

   # Note: if read == 0 should set EOF flag
   ret cast.u32to64(gl_readOnce_read)
..

pub read(handle ptr, buff u8[], n u64) !u64:
   # happy path (short string)
   # should help optimize if size is known at comptime
   if n <= 0xFFFFFFFF:
      ret try readOnce(handle, slices.toPtr(buff), cast.u64to32(n))
   ..

   bound u64 = n
   p ptr = slices.toPtr(buff)

   if n == 0:
      ret 0
   ..

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
      read u64 = try readOnce(handle, next, toRead)

      total = total + read

      if read < cast.u32to64(toRead):
         break
      ..
   ..
   ret total
..

pub stdout() writer.Writer:
   if cast.ptou(gl_stdout_handle) == 0:
      gl_stdout_handle = ext_win32_GetStdHandle(-11)
   ..

   wr writer.Writer
   wr.impl = gl_stdout_handle
   wr.fn_write = write
   ret wr
..

pub stderr() writer.Writer:
   if cast.ptou(gl_stderr_handle) == 0:
      gl_stderr_handle = ext_win32_GetStdHandle(-12)
   ..

   wr writer.Writer
   wr.impl = gl_stderr_handle
   wr.fn_write = write
   ret wr
..

pub stdin() reader.Reader:
   if cast.ptou(gl_stdin_handle) == 0:
      gl_stdin_handle = ext_win32_GetStdHandle(-10)
   ..

   rr reader.Reader
   rr.impl = gl_stdin_handle
   rr.fn_read = read
   ret rr
..

pub closeFile(handle ptr) void:
   ext_win32_CloseHandle(handle)
..

pub openFile(a alc.Allocator, path str, openMode file.OpenMode) !$ptr:
   READ  u32 = 0x80000000
   WRITE u32 = 0x40000000

   OPEN_EXISTING i32 = 3
   CREATE_ALWAYS i32 = 2

   access_mode u32
   open_mode i32

   if openMode.read:
      access_mode = READ
      if openMode.write:
         access_mode = access_mode | WRITE
      ..
      open_mode = OPEN_EXISTING
   elif openMode.write && openMode.append == false:
      access_mode = WRITE
      if openMode.read:
         access_mode = access_mode | READ
      ..
      open_mode = CREATE_ALWAYS
   elif openMode.append && openMode.write:
      access_mode = WRITE
      if openMode.read:
         access_mode = access_mode | READ
      ..
      open_mode = OPEN_EXISTING
   else:
      throw errors.errInvalidArgument("invalid open mode")
   ..

   path_u16 u16[] = try utf8.utf8To16(a, path)
   path_ptr u16* =  slices.toPtr(path_u16)

   handle ptr = ext_win32_CreateFileW(path_ptr, access_mode, 0, cast.utop(0), open_mode, 0, cast.utop(0))

   # invalid handle
   if cast.ptou(handle) == cast.itou(-1):
      if openMode.append:
         # create file if append mode
         handle = ext_win32_CreateFileW(path_ptr, access_mode, 0, cast.utop(0), 2, 0, cast.utop(0))
      ..

      if cast.ptou(handle) == cast.itou(-1):
         # TODO: map windows API errs to magma errs
         throw errors.errFailure("open failure")
      ..
   ..

   ret handle
..
