mod main

use "std:process" process
use "std:errors" errors
use "std:heap" heap
use "std:fs" fs
use "std:strings" strings
use "std:time" time
use "std:thread_pool" thread_pool

const readyFile str = "std_process_kill_ready.tmp"

fileExists(path str) bool:
    a := heap.allocator()
    contents str, readError error = fs.readFile(a, path)
    if readError.ok():
        strings.free(a, contents)
        ret true
    ..
    ret false
..

waitForChildStart() !void:
    attempts u64 = 0
    while attempts < 100 && fileExists(readyFile) == false:
        time.sleep(20)
        attempts = attempts + 1
    ..
    if fileExists(readyFile) == false:
        throw errors.failure("kill test child did not start")
    ..
..

@platform("windows")
runExecTest() !void:
    args := array str[2]
    args[0] = "/d"
    args[1] = "/c exit 7"
    code u32 = try process.exec("cmd.exe", args)
    if code != 7:
        throw errors.failure("exec returned the wrong exit code")
    ..
..

@platform("windows")
runSpawnTest() !void:
    childArgs := array str[2]
    childArgs[0] = "/d"
    childArgs[1] = "/c exit 0"
    child := try process.spawn("cmd.exe", childArgs)
    try child.isFinished()
    if try child.await() != 0:
        throw errors.failure("spawned process returned the wrong exit code")
    ..

    a := heap.allocator()
    pool := try thread_pool.new(a, 1, 1, 4, 1)
    asyncArgs := array str[2]
    asyncArgs[0] = "/d"
    asyncArgs[1] = "/c exit 11"

    execPending := try process.execAsync(pool, a, "cmd.exe", asyncArgs)
    if try execPending.await() != 11:
        try pool.close()
        throw errors.failure("execAsync returned the wrong exit code")
    ..
    try pool.close()

    cleanupArgs := array str[2]
    cleanupArgs[0] = "/d"
    cleanupArgs[1] = "/c del /q std_process_kill_ready.tmp 2>nul & exit /b 0"
    try process.exec("cmd.exe", cleanupArgs)

    # Keep the directly spawned process alive without launching a delay helper.
    # This prevents the test from orphaning ping.exe when the parent is killed.
    killArgs := array str[4]
    killArgs[0] = "-NoProfile"
    killArgs[1] = "-NonInteractive"
    killArgs[2] = "-Command"
    killArgs[3] = "[IO.File]::WriteAllText('std_process_kill_ready.tmp', 'ready'); while ($true) { Start-Sleep -Milliseconds 100 }"
    killed := try process.spawn("powershell.exe", killArgs)
    try waitForChildStart()
    try killed.kill()
    try process.exec("cmd.exe", cleanupArgs)
..

@platform("linux", "android", "ios", "darwin", "freebsd", "netbsd", "openbsd")
runExecTest() !void:
    args := array str[2]
    args[0] = "-c"
    args[1] = "exit 7"
    code u32 = try process.exec("sh", args)
    if code != 7:
        throw errors.failure("exec returned the wrong exit code")
    ..
..

@platform("linux", "android", "ios", "darwin", "freebsd", "netbsd", "openbsd")
runSpawnTest() !void:
    args := array str[2]
    args[0] = "-c"
    args[1] = "exit 0"
    child := try process.spawn("sh", args)
    try child.isFinished()
    if try child.await() != 0:
        throw errors.failure("spawned process returned the wrong exit code")
    ..

    a := heap.allocator()
    pool := try thread_pool.new(a, 1, 1, 4, 1)
    asyncArgs := array str[2]
    asyncArgs[0] = "-c"
    asyncArgs[1] = "exit 11"

    execPending := try process.execAsync(pool, a, "sh", asyncArgs)
    if try execPending.await() != 11:
        try pool.close()
        throw errors.failure("execAsync returned the wrong exit code")
    ..
    try pool.close()

    cleanupArgs := array str[2]
    cleanupArgs[0] = "-c"
    cleanupArgs[1] = "rm -f std_process_kill_ready.tmp"
    try process.exec("sh", cleanupArgs)

    # The loop is a shell builtin, so killing the shell cannot leave a sleep
    # helper behind.
    killArgs := array str[2]
    killArgs[0] = "-c"
    killArgs[1] = "printf ready > std_process_kill_ready.tmp; while :; do :; done"
    killed := try process.spawn("sh", killArgs)
    try waitForChildStart()
    try killed.kill()
    try process.exec("sh", cleanupArgs)
..

pub main() !void:
    try runExecTest()
    try runSpawnTest()
..
