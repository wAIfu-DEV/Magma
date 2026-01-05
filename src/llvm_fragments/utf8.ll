; Fragment="Utf8"
; Use="utf8 parsing essentials for use with the utf8 standard library"

define i64 @utf8.decode(ptr noundef %0, ptr noundef %1) {
  %3 = icmp eq ptr %0, null
  br i1 %3, label %100, label %4

4:
  %5 = icmp eq ptr %1, null
  br i1 %5, label %8, label %6

6:
  %7 = icmp ult ptr %0, %1
  br i1 %7, label %11, label %100

8:
  %9 = load i8, ptr %0, align 1
  %10 = icmp eq i8 %9, 0
  br i1 %10, label %100, label %13

11:
  %12 = load i8, ptr %0, align 1
  br label %13

13:
  %14 = phi i8 [ %12, %11 ], [ %9, %8 ]
  %15 = sext i8 %14 to i32
  %16 = icmp slt i8 %14, 0
  br i1 %16, label %17, label %96

17:
  %18 = and i32 %15, 224
  %19 = icmp eq i32 %18, 192
  br i1 %19, label %26, label %20

20:
  %21 = and i32 %15, 240
  %22 = icmp eq i32 %21, 224
  br i1 %22, label %26, label %23

23:
  %24 = and i32 %15, 248
  %25 = icmp eq i32 %24, 240
  br i1 %25, label %26, label %100

26:
  %27 = phi i32 [ 31, %17 ], [ 15, %20 ], [ 7, %23 ]
  %28 = phi i1 [ false, %17 ], [ true, %20 ], [ false, %23 ]
  %29 = phi i8 [ 2, %17 ], [ 3, %20 ], [ 4, %23 ]
  %30 = and i32 %27, %15
  br i1 %5, label %47, label %31

31:
  %32 = ptrtoint ptr %1 to i64
  %33 = ptrtoint ptr %0 to i64
  %34 = sub i64 %32, %33
  %35 = zext nneg i8 %29 to i64
  %36 = icmp ult i64 %34, %35
  br i1 %36, label %100, label %51

37:
  br i1 %19, label %51, label %38

38:
  %39 = getelementptr i8, ptr %0, i64 2
  %40 = load i8, ptr %39, align 1
  %41 = icmp eq i8 %40, 0
  br i1 %41, label %100, label %42

42:
  br i1 %28, label %51, label %43

43:
  %44 = getelementptr i8, ptr %0, i64 3
  %45 = load i8, ptr %44, align 1
  %46 = icmp eq i8 %45, 0
  br i1 %46, label %100, label %51

47:
  %48 = getelementptr i8, ptr %0, i64 1
  %49 = load i8, ptr %48, align 1
  %50 = icmp eq i8 %49, 0
  br i1 %50, label %100, label %37

51:
  %52 = getelementptr i8, ptr %0, i64 1
  %53 = load i8, ptr %52, align 1
  %54 = zext i8 %53 to i32
  %55 = and i32 %54, 192
  %56 = icmp eq i32 %55, 128
  br i1 %56, label %57, label %100

57:
  %58 = shl nuw nsw i32 %30, 6
  %59 = and i32 %54, 63
  %60 = or disjoint i32 %59, %58
  br i1 %19, label %81, label %61

61:
  %62 = getelementptr i8, ptr %0, i64 2
  %63 = load i8, ptr %62, align 1
  %64 = zext i8 %63 to i32
  %65 = and i32 %64, 192
  %66 = icmp eq i32 %65, 128
  br i1 %66, label %67, label %100

67:
  %68 = shl nuw nsw i32 %60, 6
  %69 = and i32 %64, 63
  %70 = or disjoint i32 %69, %68
  br i1 %28, label %81, label %71

71:
  %72 = getelementptr i8, ptr %0, i64 3
  %73 = load i8, ptr %72, align 1
  %74 = zext i8 %73 to i32
  %75 = and i32 %74, 192
  %76 = icmp eq i32 %75, 128
  br i1 %76, label %77, label %100

77:
  %78 = shl i32 %70, 6
  %79 = and i32 %74, 63
  %80 = or disjoint i32 %79, %78
  br label %81

81:
  %82 = phi i32 [ %58, %57 ], [ %68, %67 ], [ %78, %77 ]
  %83 = phi i32 [ %60, %57 ], [ %70, %67 ], [ %80, %77 ]
  %84 = phi i32 [ %30, %57 ], [ %60, %67 ], [ %70, %77 ]
  switch i8 %29, label %95 [
    i8 1, label %96
    i8 2, label %85
    i8 3, label %87
    i8 4, label %92
  ]

85:
  %86 = icmp ult i32 %82, 128
  br i1 %86, label %100, label %96

87:
  %88 = icmp ult i32 %82, 2048
  %89 = and i32 %84, 67108832
  %90 = icmp eq i32 %89, 864
  %91 = or i1 %88, %90
  br i1 %91, label %100, label %96

92:
  %93 = add i32 %82, -1114112
  %94 = icmp ult i32 %93, -1048576
  br i1 %94, label %100, label %96

95:
  unreachable

96:
  %97 = phi i32 [ %83, %92 ], [ %83, %87 ], [ %83, %85 ], [ %83, %81 ], [ %15, %13 ]
  %98 = phi i64 [ 17179869184, %92 ], [ 12884901888, %87 ], [ 8589934592, %85 ], [ 4294967296, %81 ], [ 4294967296, %13 ]
  %99 = zext i32 %97 to i64
  br label %100

100:
  %101 = phi i64 [ 0, %2 ], [ 0, %6 ], [ 0, %8 ], [ %99, %96 ], [ 0, %23 ], [ 0, %31 ], [ 0, %85 ], [ 0, %87 ], [ 0, %92 ], [ 0, %71 ], [ 0, %61 ], [ 0, %51 ], [ 0, %43 ], [ 0, %38 ], [ 0, %47 ]
  %102 = phi i64 [ 0, %2 ], [ 0, %6 ], [ 0, %8 ], [ %98, %96 ], [ 0, %23 ], [ 0, %31 ], [ 0, %85 ], [ 0, %87 ], [ 0, %92 ], [ 0, %71 ], [ 0, %61 ], [ 0, %51 ], [ 0, %43 ], [ 0, %38 ], [ 0, %47 ]
  %103 = or disjoint i64 %102, %101
  ret i64 %103
}
