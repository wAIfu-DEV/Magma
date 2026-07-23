@ECHO OFF

SET CWD = ~dp0

ECHO Building Magma Compiler Frontend...
CALL go build

if %ERRORLEVEL% GEQ 1 GOTO :End

ECHO.
ECHO Compilation success.

PAUSE
EXIT

:End

ECHO.
ECHO Compilation failed.

PAUSE
