mod file_impl_win

use "../file.mg"      file
use "../utf8.mg"      utf8
use "../allocator.mg" alc
use "../slices.mg"    slices
use "../cast.mg"      cast
use "../errors.mg"    errors

ext ext_CreateFileW   CreateFileW(pathUtf16 i16*, accessMode i32, _arg0 i32, _arg1 ptr, createMode i32, _arg2 i32, _arg3 ptr) ptr
ext ext_CloseHandle   CloseHandle(handle ptr) i32

pub write(handle ptr, bytes str) !u64:
   ret 0
..

pub read(handle ptr, n u64) !str:
   ret ""
..

pub closeFile(handle ptr) void:
   ext_CloseHandle(handle)
..

pub openFile(a alc.Allocator, path str, openMode file.OpenMode) !ptr:
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
   elif openMode.write:
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

   handle ptr = ext_CreateFileW(path_ptr, access_mode, 0, cast.utop(0), open_mode, 0, cast.utop(0))

   # invalid handle
   if cast.ptou(handle) == cast.itou(-1):
      if openMode.append:
         # create file if append mode
         handle = ext_CreateFileW(path_ptr, access_mode, 0, cast.utop(0), 2, 0, cast.utop(0))
      ..

      if cast.ptou(handle) == cast.itou(-1):
         # TODO: map windows API errs to magma errs
         throw errors.errFailure("open failure")
      ..
   ..

   ret handle
..
