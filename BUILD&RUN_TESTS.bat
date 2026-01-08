@ECHO OFF

SET CWD = ~dp0

ECHO Building Compiler Frontend...
CALL go build

if %ERRORLEVEL% GEQ 1 GOTO :End

ECHO.
ECHO Running Compiler Frontend w. file sample/tests.mg ...
CALL Magma.exe samples/tests.mg

if %ERRORLEVEL% GEQ 1 GOTO :End

ECHO.
ECHO Running Compiler Backend ...
CALL clang.exe out.ll -o out.exe

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
