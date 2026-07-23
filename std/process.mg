mod process
# Starts, waits for, and asynchronously executes child processes.

use "std:allocator" allocator
use "std:future" future
use "std:thread_pool" thread_pool

@platform("windows")
use "std:win/process_impl" impl_process

@platform("linux", "android", "ios", "darwin", "freebsd", "netbsd", "openbsd")
use "std:unix/process_impl" impl_process

# A spawned child process. A Process owns its native process resource and must
# be waited exactly once.
pub Process(
    impl impl_process.Process
)

SpawnTask(
    executable str
    arguments str[]
)

# Starts executable with arguments. The executable becomes argv[0], so callers
# should not repeat it in arguments. The child inherits the parent's environment
# and standard streams.
# @param executable executable path or name
# @param arguments arguments following argv[0]
# @returns owned running process handle
# @mustcall await or kill
# @example
#   child := try process.spawn("tool", arguments)
#   exitCode := try child.await()
pub spawn(executable str, arguments str[]) !$Process:
    child := try impl_process.spawn(executable, arguments)
    ret Process(impl=child)
..

# Returns true when the child has exited. This does not release the Process;
# await must still be called afterwards.
# @example
#   finished := try child.isFinished()
Process.isFinished() !bool:
    ret try impl_process.isFinished(addrof this.impl)
..

# Waits for the child, releases its native resource, and returns its exit code.
# On Unix, signal termination is reported as 128 plus the signal number.
# @returns child exit code
# @ownership Consumes the process handle.
destr Process.await() !u32:
    ret try impl_process.await(addrof this.impl)
..

# Terminates the child if it is still running and releases its native resource.
# No exit code is returned. Use await when normal completion matters.
# @ownership Consumes the process handle.
destr Process.kill() !void:
    try impl_process.kill(addrof this.impl)
..

# Starts executable, waits for it to finish, and returns its exit code.
# @param executable executable path or name
# @param arguments arguments following argv[0]
# @returns child exit code
# @example
#   exitCode := try process.exec("tool", arguments)
pub exec(executable str, arguments str[]) !u32:
    child := try spawn(executable, arguments)
    ret try child.await()
..

runExecTask(task SpawnTask*) !u32:
    ret try exec(task.executable, task.arguments)
..

# Runs exec on the supplied pool and resolves to the child's exit code. The
# executable and argument slice are borrowed and must remain valid until await.
# @param pool pool used to run the blocking process operation
# @param a allocator for future state
# @param executable executable path or name
# @param arguments arguments following argv[0]
# @returns owned future resolving to the child exit code
# @ownership The future must be awaited or freed according to the future API.
# @example
#   pending := try process.execAsync(pool, a, "tool", arguments)
pub execAsync(pool thread_pool.ThreadPool, a allocator.Allocator, executable str, arguments str[]) !$future.Future[u32]:
    task := SpawnTask(executable=executable, arguments=arguments)
    ret try future.new[u32, SpawnTask](a, pool, runExecTask, task)
..
