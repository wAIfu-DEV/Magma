@ECHO OFF

SET CWD = ~dp0

ECHO Building...
CALL go build

ECHO Running w. file sample/minimal.mg
CALL Magma.exe samples/minimal.mg

PAUSE
