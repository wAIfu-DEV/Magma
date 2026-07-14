@ECHO OFF

SET CWD = ~dp0

ECHO Building...
CALL go build

ECHO Building executable...
CALL Magma.exe --emit exe --out out.exe samples/http_get.mg
CALL out.exe

PAUSE
