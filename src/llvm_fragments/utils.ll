; Fragment="Utils"
; Use="miscellaneous utilities for bootstrapping magma"

declare void @llvm.memset.p0i8.i64(ptr, i8, i64, i32, i1)
declare i64 @strlen(ptr nocapture readonly) nounwind
declare i32 @printf(ptr, ...)

; Error traces use a bounded, reusable 64-way sharded ring. The 16-bit handle
; stores a 6-bit shard and 10-bit slot. Handle zero is reserved for no trace;
; the otherwise corresponding physical slot is skipped. Cursors add a bounded
; step count so reused parent links can never make traversal loop forever.
@magma.error.trace.next.shard = internal global i64 0, align 8
@magma.error.trace.thread.shard = internal thread_local global i64 -1, align 8
@magma.error.trace.shards = internal global [64 x %type.error.trace.shard] zeroinitializer, align 64
%type.error.trace.arena = type { [{{TRACE_SLOTS}} x %type.error.trace.node], [{{TRACE_ARENA_PADDING}} x i8] }
@magma.error.trace.nodes = internal global [64 x %type.error.trace.arena] zeroinitializer, align 64
@magma.error.fmt = private constant [27 x i8] c"Uncaught Error: %u '%.*s'\0A\00"
@magma.error.trace.fmt = private constant [20 x i8] c"  at %s (%s:%u:%u)\0A\00"
@magma.error.trace.truncated = private constant [{{TRACE_WARNING_LEN}} x i8] c"{{TRACE_WARNING}}"

define internal i64 @magma.error.trace.shard() cold noinline {
entry:
    %current = load i64, ptr @magma.error.trace.thread.shard, align 8
    %assigned = icmp ne i64 %current, -1
    br i1 %assigned, label %done, label %assign

assign:
    %ticket = atomicrmw add ptr @magma.error.trace.next.shard, i64 1 monotonic
    %new = and i64 %ticket, 63
    store i64 %new, ptr @magma.error.trace.thread.shard, align 8
    br label %done

done:
    %shard = phi i64 [ %current, %entry ], [ %new, %assign ]
    ret i64 %shard
}

define internal i64 @magma.error.trace.capacity() cold noinline {
entry:
    ret i64 {{TRACE_SLOTS}}
}

define internal %type.error @magma.error.push(%type.error %error, ptr %site) cold noinline {
entry:
    %shard = call i64 @magma.error.trace.shard()
    %state = getelementptr [64 x %type.error.trace.shard], ptr @magma.error.trace.shards, i64 0, i64 %shard
    %cursor = getelementptr %type.error.trace.shard, ptr %state, i32 0, i32 0
    %lock = getelementptr %type.error.trace.shard, ptr %state, i32 0, i32 1
    br label %acquire

acquire:
    %claimed = cmpxchg ptr %lock, i8 0, i8 1 acquire monotonic
    %locked = extractvalue { i8, i1 } %claimed, 1
    br i1 %locked, label %record, label %acquire

record:
    %ticket = atomicrmw add ptr %cursor, i64 1 monotonic
    %slot = and i64 %ticket, {{TRACE_MASK}}
    %node = getelementptr [64 x %type.error.trace.arena], ptr @magma.error.trace.nodes, i64 0, i64 %shard, i32 0, i64 %slot
    %sequence.field = getelementptr %type.error.trace.node, ptr %node, i32 0, i32 0
    %parent.field = getelementptr %type.error.trace.node, ptr %node, i32 0, i32 1
    %site.field = getelementptr %type.error.trace.node, ptr %node, i32 0, i32 2
    %writing = atomicrmw add ptr %sequence.field, i32 1 acq_rel
    %parent = extractvalue %type.error %error, 2
    store atomic i16 %parent, ptr %parent.field monotonic, align 2
    store atomic ptr %site, ptr %site.field monotonic, align 8
    %published = atomicrmw add ptr %sequence.field, i32 1 release
    %shifted = shl i64 %slot, 6
    %raw = or i64 %shifted, %shard
    %encoded = add i64 %raw, 1
    %handle = trunc i64 %encoded to i16
    %reserved = icmp eq i16 %handle, 0
    br i1 %reserved, label %record, label %release

release:
    store atomic i8 0, ptr %lock release, align 1
    %traced = insertvalue %type.error %error, i16 %handle, 2
    ret %type.error %traced
}

define internal %type.error.trace.snapshot @magma.error.trace.load(i16 %handle) cold noinline {
entry:
    %empty = icmp eq i16 %handle, 0
    br i1 %empty, label %return.empty, label %lookup

lookup:
    %wide = zext i16 %handle to i64
    %raw = sub i64 %wide, 1
    %shard = and i64 %raw, 63
    %slot = lshr i64 %raw, 6
    %node = getelementptr [64 x %type.error.trace.arena], ptr @magma.error.trace.nodes, i64 0, i64 %shard, i32 0, i64 %slot
    %sequence.field = getelementptr %type.error.trace.node, ptr %node, i32 0, i32 0
    %parent.field = getelementptr %type.error.trace.node, ptr %node, i32 0, i32 1
    %site.field = getelementptr %type.error.trace.node, ptr %node, i32 0, i32 2
    %sequence.before = load atomic i32, ptr %sequence.field acquire, align 4
    %writing = and i32 %sequence.before, 1
    %valid.before = icmp eq i32 %writing, 0
    br i1 %valid.before, label %read, label %return.truncated

read:
    %parent = load atomic i16, ptr %parent.field monotonic, align 2
    %site = load atomic ptr, ptr %site.field monotonic, align 8
    %sequence.after = load atomic i32, ptr %sequence.field acquire, align 4
    %valid.after = icmp eq i32 %sequence.after, %sequence.before
    br i1 %valid.after, label %return.value, label %return.truncated

return.value:
    %v0 = insertvalue %type.error.trace.snapshot zeroinitializer, ptr %site, 0
    %v1 = insertvalue %type.error.trace.snapshot %v0, i16 %parent, 1
    ret %type.error.trace.snapshot %v1

return.empty:
    ret %type.error.trace.snapshot zeroinitializer

return.truncated:
    %truncated = insertvalue %type.error.trace.snapshot zeroinitializer, i1 true, 2
    ret %type.error.trace.snapshot %truncated
}

define internal i64 @magma.error.trace(%type.error %error) cold noinline {
entry:
    %handle = extractvalue %type.error %error, 2
    %cursor = zext i16 %handle to i64
    ret i64 %cursor
}

define internal i32 @magma.error.trace.status(i64 %cursor) cold noinline {
entry:
    %truncated.bits = and i64 %cursor, 4294967296
    %truncated = icmp ne i64 %truncated.bits, 0
    %handle = trunc i64 %cursor to i16
    %empty = icmp eq i16 %handle, 0
    %empty.status = select i1 %empty, i32 1, i32 0
    %status = select i1 %truncated, i32 2, i32 %empty.status
    ret i32 %status
}

define internal i64 @magma.error.trace.next(i64 %cursor) cold noinline {
entry:
    %handle = trunc i64 %cursor to i16
    %snapshot = call %type.error.trace.snapshot @magma.error.trace.load(i16 %handle)
    %truncated = extractvalue %type.error.trace.snapshot %snapshot, 2
    %parent = extractvalue %type.error.trace.snapshot %snapshot, 1
    %count.shifted = lshr i64 %cursor, 16
    %count = and i64 %count.shifted, 65535
    %next.count = add i64 %count, 1
    %at.limit = icmp uge i64 %next.count, {{TRACE_SLOTS}}
    %has.parent = icmp ne i16 %parent, 0
    %exhausted = and i1 %at.limit, %has.parent
    %invalid = or i1 %truncated, %exhausted
    %wide.parent = zext i16 %parent to i64
    %packed.count = shl i64 %next.count, 16
    %packed = or i64 %packed.count, %wide.parent
    %next = select i1 %invalid, i64 4294967296, i64 %packed
    ret i64 %next
}

define internal %type.str @magma.error.trace.function(i64 %cursor) cold noinline {
entry:
    %handle = trunc i64 %cursor to i16
    %snapshot = call %type.error.trace.snapshot @magma.error.trace.load(i16 %handle)
    %site = extractvalue %type.error.trace.snapshot %snapshot, 0
    %valid = icmp ne ptr %site, null
    br i1 %valid, label %read, label %invalid

read:
    %field = getelementptr %type.error.site, ptr %site, i32 0, i32 0
    %value = load ptr, ptr %field
    %length = call i64 @strlen(ptr %value)
    %s0 = insertvalue %type.str zeroinitializer, ptr %value, 0
    %s1 = insertvalue %type.str %s0, i64 %length, 1
    ret %type.str %s1

invalid:
    ret %type.str zeroinitializer
}

define internal %type.str @magma.error.trace.file(i64 %cursor) cold noinline {
entry:
    %handle = trunc i64 %cursor to i16
    %snapshot = call %type.error.trace.snapshot @magma.error.trace.load(i16 %handle)
    %site = extractvalue %type.error.trace.snapshot %snapshot, 0
    %valid = icmp ne ptr %site, null
    br i1 %valid, label %read, label %invalid

read:
    %field = getelementptr %type.error.site, ptr %site, i32 0, i32 1
    %value = load ptr, ptr %field
    %length = call i64 @strlen(ptr %value)
    %s0 = insertvalue %type.str zeroinitializer, ptr %value, 0
    %s1 = insertvalue %type.str %s0, i64 %length, 1
    ret %type.str %s1

invalid:
    ret %type.str zeroinitializer
}

define internal i32 @magma.error.trace.line(i64 %cursor) cold noinline {
entry:
    %handle = trunc i64 %cursor to i16
    %snapshot = call %type.error.trace.snapshot @magma.error.trace.load(i16 %handle)
    %site = extractvalue %type.error.trace.snapshot %snapshot, 0
    %valid = icmp ne ptr %site, null
    br i1 %valid, label %read, label %invalid

read:
    %field = getelementptr %type.error.site, ptr %site, i32 0, i32 2
    %value = load i32, ptr %field
    ret i32 %value

invalid:
    ret i32 0
}

define internal i32 @magma.error.trace.column(i64 %cursor) cold noinline {
entry:
    %handle = trunc i64 %cursor to i16
    %snapshot = call %type.error.trace.snapshot @magma.error.trace.load(i16 %handle)
    %site = extractvalue %type.error.trace.snapshot %snapshot, 0
    %valid = icmp ne ptr %site, null
    br i1 %valid, label %read, label %invalid

read:
    %field = getelementptr %type.error.site, ptr %site, i32 0, i32 3
    %value = load i32, ptr %field
    ret i32 %value

invalid:
    ret i32 0
}

define internal void @magma.error.printTrace(%type.error %error) cold noinline {
entry:
    %head = extractvalue %type.error %error, 2
    br label %loop

loop:
    %handle = phi i16 [ %head, %entry ], [ %parent, %body ]
    %count = phi i64 [ 0, %entry ], [ %next.count, %body ]
    %done = icmp eq i16 %handle, 0
    br i1 %done, label %finish, label %load

load:
    %snapshot = call %type.error.trace.snapshot @magma.error.trace.load(i16 %handle)
    %parent = extractvalue %type.error.trace.snapshot %snapshot, 1
    %site = extractvalue %type.error.trace.snapshot %snapshot, 0
    %truncated = extractvalue %type.error.trace.snapshot %snapshot, 2
    br i1 %truncated, label %warn, label %body

body:
    %function.field = getelementptr %type.error.site, ptr %site, i32 0, i32 0
    %file.field = getelementptr %type.error.site, ptr %site, i32 0, i32 1
    %line.field = getelementptr %type.error.site, ptr %site, i32 0, i32 2
    %column.field = getelementptr %type.error.site, ptr %site, i32 0, i32 3
    %function = load ptr, ptr %function.field
    %file = load ptr, ptr %file.field
    %line = load i32, ptr %line.field
    %column = load i32, ptr %column.field
    call i32 (ptr, ...) @printf(ptr @magma.error.trace.fmt, ptr %function, ptr %file, i32 %line, i32 %column)
    %next.count = add i64 %count, 1
    %has.parent = icmp ne i16 %parent, 0
    %at.limit = icmp uge i64 %next.count, {{TRACE_SLOTS}}
    %exhausted = and i1 %has.parent, %at.limit
    br i1 %exhausted, label %warn, label %loop

warn:
    call i32 (ptr, ...) @printf(ptr @magma.error.trace.truncated)
    br label %finish

finish:
    ret void
}

define internal void @magma.error.print(%type.error %error) cold noinline {
entry:
    %message = extractvalue %type.error %error, 0
    %code = extractvalue %type.error %error, 1
    %length = extractvalue %type.error %error, 3
    %length32 = zext i16 %length to i32
    call i32 (ptr, ...) @printf(ptr @magma.error.fmt, i32 %code, i32 %length32, ptr %message)
    call void @magma.error.printTrace(%type.error %error)
    ret void
}

; converts a argc, argv pair into a magma slice of str
; only needed up to the point where:
; - array indexing implemented
; - allocators implemented
; - string ops (strlen) implemented
define internal %type.slice @magma.argsToSlice(i32 %argc, ptr %argv, ptr %buf) {
enter:
    %argc64 = sext i32 %argc to i64
    br label %loop

loop:
    ; for i = 0..argc {
    %i = phi i64 [0, %enter], [%i.next, %loop.body]
    %done = icmp eq i64 %i, %argc64
    br i1 %done, label %finish, label %loop.body

loop.body:
    ; cstr = argv[i]
    ; len = strlen(cstr)
    %argptrptr = getelementptr ptr, ptr %argv, i64 %i
    %cstr      = load ptr, ptr %argptrptr
    %len       = call i64 @strlen(ptr %cstr)

    ; buf[i] = str{ cstr, len }
    %elem = getelementptr %type.str, ptr %buf, i64 %i
    %f0 = insertvalue %type.str zeroinitializer, ptr %cstr, 0
    %f1 = insertvalue %type.str %f0, i64 %len, 1
    store %type.str %f1, ptr %elem

    ; i++
    ; }
    %i.next = add i64 %i, 1
    br label %loop

finish:
    ; return slice{ buf, argc }
    %slice0 = insertvalue %type.slice zeroinitializer, ptr %buf, 0
    %slice1 = insertvalue %type.slice %slice0, i64 %argc64, 1
    ret %type.slice %slice1
}
