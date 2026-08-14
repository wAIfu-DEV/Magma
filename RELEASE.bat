@ECHO OFF
SETLOCAL

CD /D "%~dp0"
SET "GOCACHE=%~dp0.gocache"

WHERE go >NUL 2>NUL
IF ERRORLEVEL 1 (
    ECHO Go was not found on PATH. 1>&2
    EXIT /B 1
)

go build -trimpath -o "%~dp0Magma.exe" .
IF ERRORLEVEL 1 (
    ECHO Compiler build failed. 1>&2
    EXIT /B 1
)

ECHO Running the release test suite...
CALL "%~dp0RUN_TESTS.bat"
IF ERRORLEVEL 1 (
    ECHO Test suite failed. Release aborted. 1>&2
    EXIT /B 1
)

ECHO Cross-compiling Windows amd64 release...
SET "GOOS=windows"
SET "GOARCH=amd64"
go build -trimpath -o "%~dp0Magma.exe" .
IF ERRORLEVEL 1 (
    ECHO Windows amd64 build failed. 1>&2
    EXIT /B 1
)

ECHO Cross-compiling Linux amd64 release...
SET "GOOS=linux"
go build -trimpath -o "%~dp0Magma" .
IF ERRORLEVEL 1 (
    ECHO Linux amd64 build failed. 1>&2
    EXIT /B 1
)
SET "GOOS="
SET "GOARCH="

FOR /F "usebackq delims=" %%V IN (`powershell -NoProfile -Command "$v = (Get-Content -Raw -LiteralPath 'VERSION.txt').Trim(); if ($v -notmatch '^(\d+)\.(\d+)\.(\d+)$') { Write-Error 'VERSION.txt must contain a semantic version such as 1.2.3'; exit 1 }; '{0}.{1}.{2}' -f $Matches[1], $Matches[2], ([int]$Matches[3] + 1)"`) DO SET "VERSION=%%V"
IF NOT DEFINED VERSION EXIT /B 1

>VERSION.txt ECHO %VERSION%
ECHO Releasing Magma %VERSION%...

SET "ARCHIVE=magma-%VERSION%-x86_64-pc-windows.zip"
SET "BINARY=Magma.exe"
CALL :CreateArchive
IF ERRORLEVEL 1 (
    EXIT /B 1
)
ECHO Created %ARCHIVE%

SET "ARCHIVE=magma-%VERSION%-x86_64-unknown-linux.zip"
SET "BINARY=Magma"
CALL :CreateArchive
IF ERRORLEVEL 1 EXIT /B 1
ECHO Created %ARCHIVE%
EXIT /B 0

:CreateArchive
powershell -NoProfile -Command "$ErrorActionPreference = 'Stop'; $root = (Get-Location).Path; $archive = Join-Path $root $env:ARCHIVE; $patterns = Get-Content -LiteralPath 'RELEASE_IGNORE.txt' | ForEach-Object { $_.Trim() } | Where-Object { $_ -and -not $_.StartsWith('#') }; $stage = Join-Path ([IO.Path]::GetTempPath()) ('magma-release-' + [guid]::NewGuid()); try { New-Item -ItemType Directory -Path $stage | Out-Null; Get-ChildItem -LiteralPath $root -Recurse -File -Force | ForEach-Object { $relative = $_.FullName.Substring($root.Length + 1).Replace('\', '/'); $ignored = $relative -eq 'Magma' -or $relative -eq 'Magma.exe'; foreach ($pattern in $patterns) { if ($relative -like $pattern) { $ignored = $true; break } }; if (-not $ignored) { $destination = Join-Path $stage $relative; New-Item -ItemType Directory -Path (Split-Path $destination) -Force | Out-Null; Copy-Item -LiteralPath $_.FullName -Destination $destination } }; Copy-Item -LiteralPath (Join-Path $root $env:BINARY) -Destination (Join-Path $stage $env:BINARY); Compress-Archive -Path (Join-Path $stage '*') -DestinationPath $archive -CompressionLevel Optimal -Force } finally { if (Test-Path -LiteralPath $stage) { Remove-Item -LiteralPath $stage -Recurse -Force } }"
IF ERRORLEVEL 1 (
    ECHO Archive creation failed for %ARCHIVE%. 1>&2
    EXIT /B 1
)
EXIT /B 0
