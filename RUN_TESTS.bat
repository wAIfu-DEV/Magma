@ECHO OFF
SETLOCAL EnableExtensions DisableDelayedExpansion

SET "ROOT=%~dp0"
SET "TEST_ROOT=%~dp0tests"
SET "STD_TEST_ROOT=%~dp0std\tests"
SET "COMPILER=%~dp0Magma.exe"
SET "GOCACHE=%~dp0.gocache"
SET "LOG_FILE=%TEMP%\magma-tests-%RANDOM%-%RANDOM%.log"
SET "OUTPUT_FILE=%TEMP%\magma-tests-%RANDOM%-%RANDOM%.ll"
SET "EXECUTABLE_FILE=%TEMP%\magma-tests-%RANDOM%-%RANDOM%.exe"
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
IF NOT EXIST "%STD_TEST_ROOT%\" (
    ECHO [FAIL] Standard library test directory not found: "%STD_TEST_ROOT%"
    PAUSE
    EXIT /B 1
)

FOR %%M IN ("%ROOT%std\*.mg") DO (
    IF /I NOT "%%~nxM"=="raylib.mg" IF NOT EXIST "%STD_TEST_ROOT%\%%~nxM" (
        ECHO [FAIL] Missing standard library test: "%STD_TEST_ROOT%\%%~nxM"
        PAUSE
        EXIT /B 1
    )
)
FOR %%T IN ("%STD_TEST_ROOT%\*.mg") DO (
    IF NOT EXIST "%ROOT%std\%%~nxT" (
        ECHO [FAIL] Standard library test has no matching module: "%%~fT"
        PAUSE
        EXIT /B 1
    )
)

SET "SHOW_OUTPUT=N"
SET /P "SHOW_OUTPUT=Display compilation output? [y/N]: "
IF /I NOT "%SHOW_OUTPUT%"=="Y" SET "SHOW_OUTPUT=N"

ECHO.
ECHO Running Magma compilation tests...
SET "TEST_SUITE_ROOT=%TEST_ROOT%"
SET "RUN_ASSERTIONS=N"
FOR /R "%TEST_ROOT%" %%F IN (*.mg) DO CALL :RunOne "%%~fF"

ECHO.
ECHO Running standard library compilation and assertion tests...
SET "TEST_SUITE_ROOT=%STD_TEST_ROOT%"
SET "RUN_ASSERTIONS=Y"
FOR /R "%STD_TEST_ROOT%" %%F IN (*.mg) DO CALL :RunOne "%%~fF"

DEL /Q "%LOG_FILE%" "%OUTPUT_FILE%" "%EXECUTABLE_FILE%" >NUL 2>&1
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
IF /I "%CHECK_DIR%"=="%TEST_SUITE_ROOT%" GOTO ExpectationFound
FOR %%D IN ("%CHECK_DIR%\..") DO SET "CHECK_DIR=%%~fD"
GOTO FindExpectation

:ExpectationFound
DEL /Q "%OUTPUT_FILE%" >NUL 2>&1
DEL /Q "%EXECUTABLE_FILE%" >NUL 2>&1
CALL :GetTimeCs COMPILE_START
IF /I "%RUN_ASSERTIONS%"=="Y" (
    "%COMPILER%" --emit exe --out "%EXECUTABLE_FILE%" "%TEST_FILE%" >"%LOG_FILE%" 2>&1
) ELSE (
    "%COMPILER%" --emit llvm --out "%OUTPUT_FILE%" "%TEST_FILE%" >"%LOG_FILE%" 2>&1
)
SET "COMPILE_EXIT=%ERRORLEVEL%"
CALL :GetTimeCs COMPILE_END
SET /A COMPILE_TIME=(COMPILE_END-COMPILE_START)*10
IF %COMPILE_TIME% LSS 0 SET /A COMPILE_TIME+=86400000
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
            ECHO [FAILURE] %TEST_FILE% %COMPILE_TIME% ms - rejected by a compiler crash or backend failure.
            IF /I NOT "%SHOW_OUTPUT%"=="Y" TYPE "%LOG_FILE%"
            GOTO :EOF
        )
        SET /A PASSED+=1
        ECHO [PASS] %TEST_FILE% %COMPILE_TIME% ms ^(rejected as expected^)
        GOTO :EOF
    )
    SET /A FAILED+=1
    ECHO [FAILURE] %TEST_FILE% %COMPILE_TIME% ms - compiled successfully; expected rejection.
    GOTO :EOF
)

IF "%COMPILE_EXIT%"=="0" (
    IF /I "%RUN_ASSERTIONS%"=="Y" GOTO RunAssertions
    SET /A PASSED+=1
    ECHO [PASS] %TEST_FILE% %COMPILE_TIME% ms
    GOTO :EOF
)

SET /A FAILED+=1
ECHO [FAILURE] %TEST_FILE% %COMPILE_TIME% ms - failed to compile; expected success.
IF /I NOT "%SHOW_OUTPUT%"=="Y" TYPE "%LOG_FILE%"
ECHO.
GOTO :EOF

:RunAssertions
CALL :GetTimeCs RUN_START
"%EXECUTABLE_FILE%" >>"%LOG_FILE%" 2>&1
SET "RUN_EXIT=%ERRORLEVEL%"
CALL :GetTimeCs RUN_END
SET /A RUN_TIME=(RUN_END-RUN_START)*10
IF %RUN_TIME% LSS 0 SET /A RUN_TIME+=86400000
IF "%RUN_EXIT%"=="0" (
    SET /A PASSED+=1
    ECHO [PASS] %TEST_FILE% %COMPILE_TIME% ms compile, %RUN_TIME% ms assertions
    GOTO :EOF
)

SET /A FAILED+=1
ECHO [FAILURE] %TEST_FILE% %RUN_TIME% ms - assertions failed with exit code %RUN_EXIT%.
TYPE "%LOG_FILE%"
ECHO.
GOTO :EOF

:GetTimeCs
SET "NOW=%TIME: =0%"
SET /A "%~1=(1%NOW:~0,2%-100)*360000+(1%NOW:~3,2%-100)*6000+(1%NOW:~6,2%-100)*100+(1%NOW:~9,2%-100)"
GOTO :EOF
