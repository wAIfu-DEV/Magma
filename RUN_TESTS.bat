@ECHO OFF
SETLOCAL EnableExtensions DisableDelayedExpansion

SET "ROOT=%~dp0"
SET "TEST_ROOT=%~dp0tests"
SET "COMPILER=%~dp0Magma.exe"
SET "GOCACHE=%~dp0.gocache"
SET "LOG_FILE=%TEMP%\magma-tests-%RANDOM%-%RANDOM%.log"
SET "OUTPUT_FILE=%TEMP%\magma-tests-%RANDOM%-%RANDOM%.ll"
SET /A TOTAL=0, PASSED=0, FAILED=0

ECHO Building Magma...
go build -o "%COMPILER%" "%ROOT%."
IF ERRORLEVEL 1 (
    ECHO [FAIL] Could not build the Magma compiler.
    PAUSE
    EXIT /B 1
)

IF NOT EXIST "%TEST_ROOT%\" (
    ECHO [FAIL] Test directory not found: "%TEST_ROOT%"
    PAUSE
    EXIT /B 1
)

SET "SHOW_OUTPUT=N"
SET /P "SHOW_OUTPUT=Display compilation output? [y/N]: "
IF /I NOT "%SHOW_OUTPUT%"=="Y" SET "SHOW_OUTPUT=N"

ECHO.
ECHO Running Magma compilation tests...
FOR /R "%TEST_ROOT%" %%F IN (*.mg) DO CALL :RunOne "%%~fF"

DEL /Q "%LOG_FILE%" "%OUTPUT_FILE%" >NUL 2>&1
ECHO.
ECHO Results: %PASSED% passed, %FAILED% failed, %TOTAL% total.

IF %TOTAL% EQU 0 (
    ECHO [FAIL] No .mg test files were found.
    PAUSE
    EXIT /B 1
)
IF %FAILED% GTR 0 (
    PAUSE
    EXIT /B 1
)
PAUSE
EXIT /B 0

:RunOne
SET /A TOTAL+=1
SET "TEST_FILE=%~f1"
SET "EXPECT=success"
SET "CHECK_DIR=%~dp1"

:FindExpectation
IF EXIST "%CHECK_DIR%.expect-failure" SET "EXPECT=failure"
FOR %%D IN ("%CHECK_DIR%.") DO SET "CHECK_DIR=%%~fD"
IF /I "%CHECK_DIR%"=="%TEST_ROOT%" GOTO ExpectationFound
FOR %%D IN ("%CHECK_DIR%\..") DO SET "CHECK_DIR=%%~fD"
GOTO FindExpectation

:ExpectationFound
DEL /Q "%OUTPUT_FILE%" >NUL 2>&1
"%COMPILER%" --emit llvm --out "%OUTPUT_FILE%" "%TEST_FILE%" >"%LOG_FILE%" 2>&1
SET "COMPILE_EXIT=%ERRORLEVEL%"
IF /I "%SHOW_OUTPUT%"=="Y" (
    ECHO.
    ECHO ----- Compiler output: %TEST_FILE% -----
    TYPE "%LOG_FILE%"
    ECHO ----- End compiler output -----
)

IF /I "%EXPECT%"=="failure" (
    IF NOT "%COMPILE_EXIT%"=="0" (
        FINDSTR /I /C:"panic:" /C:"goroutine " /C:"uncaught fatal error" /C:"Clang failed" /C:"fatal error in file" /C:"internal compiler error" "%LOG_FILE%" >NUL
        IF NOT ERRORLEVEL 1 (
            SET /A FAILED+=1
            ECHO [FAIL] %TEST_FILE% was rejected by a compiler crash or backend failure.
            IF /I NOT "%SHOW_OUTPUT%"=="Y" TYPE "%LOG_FILE%"
            GOTO :EOF
        )
        SET /A PASSED+=1
        ECHO [PASS] %TEST_FILE% ^(rejected as expected^)
        GOTO :EOF
    )
    SET /A FAILED+=1
    ECHO [FAIL] %TEST_FILE% compiled successfully; expected rejection.
    GOTO :EOF
)

IF "%COMPILE_EXIT%"=="0" (
    SET /A PASSED+=1
    ECHO [PASS] %TEST_FILE%
    GOTO :EOF
)

SET /A FAILED+=1
ECHO [FAIL] %TEST_FILE% failed to compile; expected success.
IF /I NOT "%SHOW_OUTPUT%"=="Y" TYPE "%LOG_FILE%"
ECHO.
GOTO :EOF
