@ECHO OFF

CALL clang.exe -O3 out.ll -S -emit-llvm -o out_o3.ll

