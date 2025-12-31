@ECHO OFF

SET CWD = ~dp0

ECHO Building Compiler Frontend...
CALL go build

if %ERRORLEVEL% GEQ 1 GOTO :End

ECHO.
ECHO Running Compiler Frontend w. file sample/minimal.mg ...
CALL Magma.exe samples/minimal.mg

if %ERRORLEVEL% GEQ 1 GOTO :End

ECHO.
ECHO Running Compiler Backend ...
CALL clang.exe -O1 out.ll -o out.exe
CALL clang.exe -O1 out.ll -S -emit-llvm -o out_o1.ll

if %ERRORLEVEL% GEQ 1 GOTO :End

ECHO.
ECHO Running out.exe
CALL out.exe "this is second arg" 10051

PAUSE
EXIT

:End

ECHO.
ECHO Compilation failed.

PAUSE
