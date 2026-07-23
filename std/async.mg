mod async
# Asynchronous I/O operations scheduled on a thread pool.
# @ownership The pool and allocator are borrowed until all operations complete.

use "std:allocator" allocator
use "std:future" future
use "std:reader" reader
use "std:thread_pool" thread_pool

# Execution context shared by asynchronous standard-library operations. Async
# borrows both dependencies; they must remain valid until all futures complete.
pub Async(
    pool thread_pool.ThreadPool
    allocator allocator.Allocator
)

ReaderReadTask(
    source reader.Reader
    allocator allocator.Allocator
    count u64
)

# Creates a lightweight asynchronous execution context. It does not own the
# pool or allocator and may be copied.
# @complexity O(1)
# @ownership pool and a must outlive every future created from this context.
# @example
#   asc := async.new(pool, a)
pub new(pool thread_pool.ThreadPool, a allocator.Allocator) Async:
    ret Async(pool=pool, allocator=a)
..

runReadTask(task ReaderReadTask*) !$str:
    ret try task.source.read(task.allocator, task.count)
..

# Reads up to nBytes on this context's pool. The Reader interface is copied
# into task storage, but its underlying implementation remains borrowed and
# must stay valid until the returned future is awaited.
# @complexity O(1) to schedule; O(N) work for the requested byte count
# @param source reader used by the worker task
# @param nBytes maximum bytes to read
# @returns owned future resolving to an owned string
# @example
#   pending := try asc.read(input, 4096)
#   contents := try pending.await()
Async.read(source reader.Reader, nBytes u64) !$future.Future[str]:
    task := ReaderReadTask(source=source, allocator=this.allocator, count=nBytes)
    ret try future.new[str, ReaderReadTask](this.allocator, this.pool, runReadTask, task)
..
