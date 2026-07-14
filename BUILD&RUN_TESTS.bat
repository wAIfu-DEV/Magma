@ECHO OFF

SET CWD = ~dp0

ECHO Building Compiler Frontend...
CALL go build

if %ERRORLEVEL% GEQ 1 GOTO :End

ECHO.
ECHO Compiling sample/tests.mg to out.exe ...
CALL Magma.exe --emit exe --out out.exe samples/tests.mg

if %ERRORLEVEL% GEQ 1 GOTO :End

ECHO.
ECHO Running out.exe
CALL out.exe

PAUSE
EXIT

:End

ECHO.
ECHO Compilation failed.

PAUSE
