@ECHO OFF
SETLOCAL

CD /D "%~dp0"
SET "GOCACHE=%~dp0.gocache"
SET "ARCH="

WHERE go >NUL 2>NUL
IF ERRORLEVEL 1 (
    ECHO Go was not found on PATH. 1>&2
    EXIT /B 1
)

FOR /F "usebackq delims=" %%V IN (`powershell -NoProfile -Command "$v = (Get-Content -Raw -LiteralPath 'VERSION.txt').Trim(); if ($v -notmatch '^(\d+)\.(\d+)\.(\d+)$') { Write-Error 'VERSION.txt must contain a semantic version such as 1.2.3'; exit 1 }; '{0}.{1}.{2}' -f $Matches[1], $Matches[2], ([int]$Matches[3] + 1)"`) DO SET "VERSION=%%V"
IF NOT DEFINED VERSION EXIT /B 1

>VERSION.txt ECHO %VERSION%
ECHO Releasing Magma %VERSION%...

go build -trimpath -o "%~dp0Magma.exe" .
IF ERRORLEVEL 1 (
    ECHO Compiler build failed. 1>&2
    EXIT /B 1
)

FOR /F "delims=" %%P IN ('go env GOARCH') DO SET "GOARCH=%%P"
IF /I "%GOARCH%"=="amd64" SET "ARCH=x86_64"
IF /I "%GOARCH%"=="386" SET "ARCH=i686"
IF /I "%GOARCH%"=="arm64" SET "ARCH=aarch64"
IF NOT DEFINED ARCH SET "ARCH=%GOARCH%"

SET "PLATFORM=%ARCH%-pc-windows"
SET "ARCHIVE=magma-%VERSION%-%PLATFORM%.zip"

powershell -NoProfile -Command "$ErrorActionPreference = 'Stop'; $root = (Get-Location).Path; $archive = Join-Path $root $env:ARCHIVE; $patterns = Get-Content -LiteralPath 'RELEASE_IGNORE.txt' | ForEach-Object { $_.Trim() } | Where-Object { $_ -and -not $_.StartsWith('#') }; $stage = Join-Path ([IO.Path]::GetTempPath()) ('magma-release-' + [guid]::NewGuid()); try { New-Item -ItemType Directory -Path $stage | Out-Null; Get-ChildItem -LiteralPath $root -Recurse -File -Force | ForEach-Object { $relative = $_.FullName.Substring($root.Length + 1).Replace('\', '/'); $ignored = $false; foreach ($pattern in $patterns) { if ($relative -like $pattern) { $ignored = $true; break } }; if (-not $ignored) { $destination = Join-Path $stage $relative; New-Item -ItemType Directory -Path (Split-Path $destination) -Force | Out-Null; Copy-Item -LiteralPath $_.FullName -Destination $destination } }; Compress-Archive -Path (Join-Path $stage '*') -DestinationPath $archive -CompressionLevel Optimal -Force } finally { if (Test-Path -LiteralPath $stage) { Remove-Item -LiteralPath $stage -Recurse -Force } }"
IF ERRORLEVEL 1 (
    ECHO Archive creation failed. 1>&2
    EXIT /B 1
)

ECHO Created %ARCHIVE%
EXIT /B 0
