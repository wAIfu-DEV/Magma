; Fragment="Utils"
; Use="miscellaneous utilities for bootstrapping magma"

declare void @llvm.memset.p0i8.i64(ptr, i8, i64, i32, i1)
declare i64 @strlen(ptr nocapture readonly) nounwind

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


; returns the updated reference count.
; used primarily by the rfc ref counting system
define i32 @magma.atomicIncMonotonic(i32* %ptr) {
  %old = atomicrmw add i32* %ptr, i32 1 monotonic
  %new = add i32 %old, 1
  ret i32 %new
}

; returns the updated reference count.
; used primarily by the rfc ref counting system
define i32 @magma.atomicDecAcqRel(i32* %ptr) {
  %old = atomicrmw sub i32* %ptr, i32 1 acq_rel
  %new = add i32 %old, -1
  ret i32 %new
}
