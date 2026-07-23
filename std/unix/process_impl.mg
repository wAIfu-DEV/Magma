mod process_impl_unix
# Unix child-process backend used by the portable process module.


use "std:c" c
use "std:heap" heap
use "std:strings" strings
use "std:slices" slices
use "std:errors" errors
use "std:cast" cast

ext ext_unix_fork fork() c.int
ext ext_unix_execvp execvp(file u8*, arguments ptr) c.int
ext ext_unix_waitpid waitpid(pid c.int, status c.int*, options c.int) c.int
ext ext_unix_kill kill(pid c.int, signal c.int) c.int
ext ext_unix_exit _exit(status c.int) void

pub Process(
    pid i32
    finished bool
    exitCode u32
)

containsNull(value str) bool:
    i u64 = 0
    while i < strings.countBytes(value):
        if strings.byteAt(value, i) == 0:
            ret true
        ..
        i = i + 1
    ..
    ret false
..

decodeStatus(status i32) u32:
    raw u64 = cast.itou(cast.i32to64(status))
    signal u32 = cast.u64to32(raw & 127)
    if signal == 0:
        ret cast.u64to32((raw >> 8) & 255)
    ..
    ret 128 + signal
..

freeArguments(argv u8**, count u64) void:
    a := heap.allocator()
    i u64 = 0
    while i < count:
        a.free(argv[i])
        i = i + 1
    ..
    a.free(argv)
..

pub spawn(executable str, arguments str[]) !$Process:
    if strings.countBytes(executable) == 0 || containsNull(executable):
        throw errors.invalidArgument("process executable is empty or contains a null byte")
    ..
    count u64 = slices.count(arguments)
    i u64 = 0
    while i < count:
        if containsNull(arguments[i]):
            throw errors.invalidArgument("process argument contains a null byte")
        ..
        i = i + 1
    ..

    a := heap.allocator()
    argv u8** = try a.allocT[u8*](count + 2)
    initialized u64 = 0
    executableCopy u8* = try strings.toCstr(a, executable)
    argv[0] = executableCopy
    initialized = 1
    i = 0
    while i < count:
        copied u8*, copyError error = strings.toCstr(a, arguments[i])
        if copyError.nok():
            freeArguments(argv, initialized)
            throw copyError
        ..
        argv[i + 1] = copied
        initialized = initialized + 1
        i = i + 1
    ..
    argv[count + 1] = none

    pid i32 = ext_unix_fork()
    if pid == 0:
        ext_unix_execvp(executableCopy, argv)
        ext_unix_exit(127)
    ..
    freeArguments(argv, initialized)
    if pid < 0:
        throw errors.failure("fork failed")
    ..
    ret Process(pid=pid, finished=false, exitCode=0)
..

pub isFinished(process Process*) !bool:
    if process.pid == 0:
        throw errors.invalidArgument("process has already been waited")
    ..
    if process.finished:
        ret true
    ..
    status i32 = 0
    result i32 = ext_unix_waitpid(process.pid, addrof status, 1)
    if result < 0:
        throw errors.failure("waitpid failed")
    elif result == 0:
        ret false
    ..
    process.exitCode = decodeStatus(status)
    process.finished = true
    ret true
..

pub await(process Process*) !u32:
    if process.pid == 0:
        throw errors.invalidArgument("process has already been waited")
    ..
    if process.finished == false:
        status i32 = 0
        result i32 = ext_unix_waitpid(process.pid, addrof status, 0)
        if result < 0:
            throw errors.failure("waitpid failed")
        ..
        process.exitCode = decodeStatus(status)
    ..
    code u32 = process.exitCode
    process.pid = 0
    process.finished = true
    ret code
..

pub kill(process Process*) !void:
    if process.pid == 0:
        throw errors.invalidArgument("process has already been consumed")
    ..
    if process.finished == false:
        if ext_unix_kill(process.pid, 9) != 0:
            throw errors.failure("kill failed")
        ..
        status i32 = 0
        if ext_unix_waitpid(process.pid, addrof status, 0) < 0:
            throw errors.failure("waitpid after kill failed")
        ..
    ..
    process.pid = 0
    process.finished = true
..
