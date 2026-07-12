@ECHO OFF

SET CWD = ~dp0

ECHO Building...
CALL go build

ECHO Running w. file sample/file_reader.mg
CALL Magma.exe samples/json_tests.mg

ECHO Running Compiler Backend ...
CALL clang.exe -O1 out.ll -S -emit-llvm -o out_o1.ll
CALL clang.exe -O3 out.ll -S -emit-llvm -o out_o3.ll

CALL clang.exe -O3 out.ll -o out.exe
CALL out.exe

PAUSE
