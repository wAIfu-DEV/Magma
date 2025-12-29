@ECHO OFF

CALL clang.exe -O1 out.ll -S -emit-llvm -o out_o1.ll

