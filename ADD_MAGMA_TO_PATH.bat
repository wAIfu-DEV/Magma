@echo off
setlocal

set "MAGMA_DIR=%~dp0"

if not exist "%MAGMA_DIR%Magma.exe" (
    echo Magma.exe was not found in "%MAGMA_DIR%". 1>&2
    pause
    exit /b 1
)

powershell.exe -NoProfile -ExecutionPolicy Bypass -Command ^
    "$dir = [IO.Path]::GetFullPath($env:MAGMA_DIR).TrimEnd([IO.Path]::DirectorySeparatorChar);" ^
    "$userPath = [Environment]::GetEnvironmentVariable('Path', 'User');" ^
    "$entries = @($userPath -split ';' | ForEach-Object { $_.Trim().TrimEnd([IO.Path]::DirectorySeparatorChar) } | Where-Object { $_ });" ^
    "if ($entries -contains $dir) { Write-Host ('Already on the user PATH: ' + $dir); exit 0 };" ^
    "$newPath = if ([string]::IsNullOrWhiteSpace($userPath)) { $dir } else { $userPath.TrimEnd(';') + ';' + $dir };" ^
    "[Environment]::SetEnvironmentVariable('Path', $newPath, 'User');" ^
    "Write-Host ('Added to the user PATH: ' + $dir)"

if errorlevel 1 (
    echo Failed to update the user PATH. 1>&2
    pause
    exit /b 1
)

echo.
echo Open a new terminal, then run: Magma.exe
pause
exit /b 0
