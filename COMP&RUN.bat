@ECHO OFF

SET CWD = ~dp0

ECHO Building ...
CALL clang.exe out.ll -o out.exe

ECHO.
ECHO Running out.exe
CALL out.exe

PAUSE
