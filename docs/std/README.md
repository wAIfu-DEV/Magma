# Magma standard-library reference

Import a module with its path under `std/`, omitting the `.mg` extension:

```magma
use "std:heap" heap
use "std:net/tcp" tcp
```

The pages in this directory document public modules and a few cross-module
topics. The source remains the exact API definition; throwing returns are
written as `!T`, and `$T` marks ownership transfer.

## Foundations and allocation

[`core`](core.md), [`errors`](errors.md), [`allocator`](allocator.md),
[`allocators`](allocators.md), [`heap`](heap.md), [`arena_alloc`](arena_alloc.md),
[`scratch_alloc`](scratch_alloc.md), [`debug_alloc`](debug_alloc.md),
[`fake_alloc`](fake_alloc.md), [`memory`](memory.md), [`cast`](cast.md),
[`checked`](checked.md), [`c`](c.md), and [`footgun`](footgun.md).

## Collections and text

[`array`](array.md), [`list`](list.md), [`queue`](queue.md),
[`linear_map`](linear_map.md), [`hash_map`](hash_map.md), [`pair`](pair.md),
[`iterator`](iterator.md), [`sort`](sort.md), [`search`](search.md),
[`slices`](slices.md), [`strings`](strings.md), [`bytes`](bytes.md),
[`builder`](builder.md), [`unicode`](unicode.md), [`utf8`](utf8.md),
[`utf16`](utf16.md), [`strconv`](strconv.md), [`fmt`](fmt.md),
[`base64`](base64.md), [`hex`](hex.md), and [`percent`](percent.md).

## I/O, system, and data formats

[`reader`](reader.md), [`writer`](writer.md), [`buffered`](buffered.md),
[`duplex`](duplex.md), [`io`](io.md), [`file`](file.md), [`file_op_mode`](file_op_mode.md),
[`fs`](fs.md), [`path`](path.md), [`process`](process.md), [`env`](env.md),
[`args`](args.md), [`flag`](flag.md), [`dialog`](dialog.md), [`time`](time.md),
[`random`](random.md), [`cpu`](cpu.md), [`json`](json.md), and [`http`](http.md).

## Concurrency and execution

[`atomic`](atomic.md), [`mutex`](mutex.md), [`spinlock`](spinlock.md),
[`locker`](locker.md), [`wake`](wake.md), [`thread`](thread.md),
[`thread_pool`](thread_pool.md), [`executor`](executor.md), [`context`](context.md),
[`future`](future.md), and the [`async`](async.md) cross-module guide.

## Networking

Start with the [networking overview](net.md). Individual references cover
[`net/address`](net-address.md), [`net/socket`](net-socket.md),
[`net/tcp`](net-tcp.md), [`net/udp`](net-udp.md), [`net/dns`](net-dns.md),
[`net/poll`](net-poll.md), [`net/event_loop`](net-event-loop.md),
[`net/listener`](net-listener.md), and [`net/tls`](net-tls.md).

## Other bindings

[`raylib`](raylib.md) documents the bundled raylib binding.

`std/vector` currently exposes no public declarations and is therefore not a
public module. Platform implementation modules under `std/win`, `std/unix`,
and `std/linux` are backend details. The older `file_dialog.md` page describes
the current `std:dialog` module; new links use [`dialog.md`](dialog.md).
