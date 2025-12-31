mod file

if platform == "windows":
    use "win/file.mg"  impl_file
elif platform == "linux" || platform == "openbsd" || platform == "netbsd" || platform == "darwin":
    use "unix/file.mg" impl_file
..

File(
    handle impl_file.FileHandle*,
    closed bool,
)

File.destructor() void:
    if this.closed == false:
        this.handle.close()
    ..
..

File.close() void:
    if this.closed == false:
        this.handle.close()
    ..
..

File.write(bytes str) !u64:
    ret impl_file.write(this.handle, bytes)
..

File.read(n u64) str:
    ret impl_file.read(this.handle, n)
..

pub openFile(path str, openMode i64) File:
    ret impl_file.openFile(path, openMode)
..
