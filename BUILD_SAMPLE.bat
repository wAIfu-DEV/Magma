@ECHO OFF

SET CWD = ~dp0

ECHO Building...
CALL go build

ECHO Building executable...

CALL Magma.exe --emit exe --out out.exe samples/args_echo.mg
if %ERRORLEVEL% GEQ 1 GOTO :End

CALL Magma.exe -O3 --emit llvm --out out.ll samples/args_echo.mg

CALL out.exe "first arg" "Héllo, World!" "you're in a coma" "it's been 10 years please wake up"

PAUSE
EXIT

:End

ECHO.
ECHO Compilation failed.

PAUSE
