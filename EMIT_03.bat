@ECHO OFF

CALL clang.exe -Ofast out.ll -S -emit-llvm -o out_o3.ll

