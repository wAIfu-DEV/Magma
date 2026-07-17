; Fragment="Utils"
; Use="miscellaneous utilities for bootstrapping magma"

declare void @llvm.memset.p0i8.i64(ptr, i8, i64, i32, i1)
declare i64 @strlen(ptr nocapture readonly) nounwind
declare i32 @printf(ptr, ...)

; Error traces use a bounded, reusable 64-way sharded ring. Handles carry the
; shard and allocation ticket so readers can detect reuse instead of following
; stale or partially-published nodes.
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

define i64 @magma.error.trace.capacity() cold noinline {
entry:
    ret i64 {{TRACE_SLOTS}}
}

define %type.error @magma.error.push(%type.error %error, ptr %site) cold noinline {
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
    %published = shl i64 %ticket, 1
    %writing = or i64 %published, 1
    store atomic i64 %writing, ptr %sequence.field release, align 8
    %parent = extractvalue %type.error %error, 1
    store atomic i64 %parent, ptr %parent.field monotonic, align 8
    store atomic ptr %site, ptr %site.field monotonic, align 8
    store atomic i64 %published, ptr %sequence.field release, align 8
    %encoded = add i64 %ticket, 1
    %shifted = shl i64 %encoded, 6
    %handle = or i64 %shifted, %shard
    store atomic i8 0, ptr %lock release, align 1
    %traced = insertvalue %type.error %error, i64 %handle, 1
    ret %type.error %traced
}

define internal %type.error.trace.snapshot @magma.error.trace.load(i64 %handle) cold noinline {
entry:
    %empty = icmp eq i64 %handle, 0
    br i1 %empty, label %return.empty, label %lookup

lookup:
    %shard = and i64 %handle, 63
    %encoded = lshr i64 %handle, 6
    %ticket = sub i64 %encoded, 1
    %slot = and i64 %ticket, {{TRACE_MASK}}
    %node = getelementptr [64 x %type.error.trace.arena], ptr @magma.error.trace.nodes, i64 0, i64 %shard, i32 0, i64 %slot
    %sequence.field = getelementptr %type.error.trace.node, ptr %node, i32 0, i32 0
    %parent.field = getelementptr %type.error.trace.node, ptr %node, i32 0, i32 1
    %site.field = getelementptr %type.error.trace.node, ptr %node, i32 0, i32 2
    %expected = shl i64 %ticket, 1
    %sequence.before = load atomic i64, ptr %sequence.field acquire, align 8
    %valid.before = icmp eq i64 %sequence.before, %expected
    br i1 %valid.before, label %read, label %return.truncated

read:
    %parent = load atomic i64, ptr %parent.field monotonic, align 8
    %site = load atomic ptr, ptr %site.field monotonic, align 8
    %sequence.after = load atomic i64, ptr %sequence.field acquire, align 8
    %valid.after = icmp eq i64 %sequence.after, %expected
    br i1 %valid.after, label %return.value, label %return.truncated

return.value:
    %v0 = insertvalue %type.error.trace.snapshot zeroinitializer, i64 %parent, 0
    %v1 = insertvalue %type.error.trace.snapshot %v0, ptr %site, 1
    ret %type.error.trace.snapshot %v1

return.empty:
    ret %type.error.trace.snapshot zeroinitializer

return.truncated:
    %truncated = insertvalue %type.error.trace.snapshot zeroinitializer, i1 true, 2
    ret %type.error.trace.snapshot %truncated
}

define i64 @magma.error.trace(%type.error %error) cold noinline {
entry:
    %handle = extractvalue %type.error %error, 1
    ret i64 %handle
}

define i32 @magma.error.trace.status(i64 %handle) cold noinline {
entry:
    %empty = icmp eq i64 %handle, 0
    br i1 %empty, label %return.empty, label %check

check:
    %snapshot = call %type.error.trace.snapshot @magma.error.trace.load(i64 %handle)
    %truncated = extractvalue %type.error.trace.snapshot %snapshot, 2
    %status = select i1 %truncated, i32 2, i32 0
    ret i32 %status

return.empty:
    ret i32 1
}

define i64 @magma.error.trace.next(i64 %handle) cold noinline {
entry:
    %snapshot = call %type.error.trace.snapshot @magma.error.trace.load(i64 %handle)
    %truncated = extractvalue %type.error.trace.snapshot %snapshot, 2
    %parent = extractvalue %type.error.trace.snapshot %snapshot, 0
    %next = select i1 %truncated, i64 1, i64 %parent
    ret i64 %next
}

define %type.str @magma.error.trace.function(i64 %handle) cold noinline {
entry:
    %snapshot = call %type.error.trace.snapshot @magma.error.trace.load(i64 %handle)
    %site = extractvalue %type.error.trace.snapshot %snapshot, 1
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

define %type.str @magma.error.trace.file(i64 %handle) cold noinline {
entry:
    %snapshot = call %type.error.trace.snapshot @magma.error.trace.load(i64 %handle)
    %site = extractvalue %type.error.trace.snapshot %snapshot, 1
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

define i32 @magma.error.trace.line(i64 %handle) cold noinline {
entry:
    %snapshot = call %type.error.trace.snapshot @magma.error.trace.load(i64 %handle)
    %site = extractvalue %type.error.trace.snapshot %snapshot, 1
    %valid = icmp ne ptr %site, null
    br i1 %valid, label %read, label %invalid

read:
    %field = getelementptr %type.error.site, ptr %site, i32 0, i32 2
    %value = load i32, ptr %field
    ret i32 %value

invalid:
    ret i32 0
}

define i32 @magma.error.trace.column(i64 %handle) cold noinline {
entry:
    %snapshot = call %type.error.trace.snapshot @magma.error.trace.load(i64 %handle)
    %site = extractvalue %type.error.trace.snapshot %snapshot, 1
    %valid = icmp ne ptr %site, null
    br i1 %valid, label %read, label %invalid

read:
    %field = getelementptr %type.error.site, ptr %site, i32 0, i32 3
    %value = load i32, ptr %field
    ret i32 %value

invalid:
    ret i32 0
}

define void @magma.error.printTrace(%type.error %error) cold noinline {
entry:
    %head = extractvalue %type.error %error, 1
    br label %loop

loop:
    %handle = phi i64 [ %head, %entry ], [ %parent, %body ]
    %done = icmp eq i64 %handle, 0
    br i1 %done, label %finish, label %load

load:
    %snapshot = call %type.error.trace.snapshot @magma.error.trace.load(i64 %handle)
    %parent = extractvalue %type.error.trace.snapshot %snapshot, 0
    %site = extractvalue %type.error.trace.snapshot %snapshot, 1
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
    br label %loop

warn:
    call i32 (ptr, ...) @printf(ptr @magma.error.trace.truncated)
    br label %finish

finish:
    ret void
}

define void @magma.error.print(%type.error %error) cold noinline {
entry:
    %message = extractvalue %type.error %error, 0
    %code = extractvalue %type.error %error, 2
    %length = extractvalue %type.error %error, 3
    call i32 (ptr, ...) @printf(ptr @magma.error.fmt, i32 %code, i32 %length, ptr %message)
    call void @magma.error.printTrace(%type.error %error)
    ret void
}

; converts a argc, argv pair into a magma slice of str
; only needed up to the point where:
; - array indexing implemented
; - allocators implemented
; - string ops (strlen) implemented
define %type.slice @magma.argsToSlice(i32 %argc, ptr %argv, ptr %buf) {
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
