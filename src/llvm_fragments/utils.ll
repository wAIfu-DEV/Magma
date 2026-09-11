; Code generated from lowering_runtime.BuildUtils; DO NOT EDIT.

declare void @llvm.memset.p0.i64(ptr writeonly captures(none), i8, i64, i1 immarg) #0

; Function Attrs: nounwind
declare i64 @strlen(ptr) #1

define internal %type.slice @magma.argsToSlice(i32 %0, ptr %1, ptr %2) {
enter:
  %3 = sext i32 %0 to i64
  br label %loop

loop:                                             ; preds = %loop.body, %enter
  %4 = phi i64 [ 0, %enter ], [ %14, %loop.body ]
  %5 = icmp eq i64 %4, %3
  br i1 %5, label %finish, label %loop.body

loop.body:                                        ; preds = %loop
  %6 = getelementptr ptr, ptr %1, i64 %4
  %7 = load ptr, ptr %6, align 8
  %8 = call i64 @strlen(ptr %7) #1
  %9 = getelementptr %type.str, ptr %2, i64 %4
  %10 = insertvalue %type.str undef, ptr %7, 0
  %11 = insertvalue %type.str %10, i64 %8, 1
  %12 = insertvalue %type.str %11, ptr null, 2
  %13 = insertvalue %type.str %12, ptr null, 3
  store %type.str %13, ptr %9, align 8
  %14 = add i64 %4, 1
  br label %loop

finish:                                           ; preds = %loop
  %15 = insertvalue %type.slice undef, ptr %2, 0
  %16 = insertvalue %type.slice %15, i64 %3, 1
  ret %type.slice %16
}

attributes #0 = { nocallback nofree nounwind willreturn memory(argmem: write) }
attributes #1 = { nounwind }
