@ECHO OFF

SET CWD = ~dp0

ECHO Building...
CALL go build

ECHO Building executable...

CALL Magma.exe --emit exe --out out.exe samples/raylib_test.mg
if %ERRORLEVEL% GEQ 1 GOTO :End

CALL Magma.exe -O3 --emit llvm --out out.ll samples/raylib_test.mg

CALL out.exe

PAUSE
EXIT

:End

ECHO.
ECHO Compilation failed.

PAUSE
