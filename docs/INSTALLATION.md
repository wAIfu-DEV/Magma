# Installing Magma

This guide installs a released Windows build of Magma. A working installation
contains `Magma.exe`, the `std` directory, and the native and dynamic libraries
shipped with that release.

## 1. Download a release archive

Open the [Magma releases page](https://github.com/wAIfu-DEV/Magma/releases),
select a release, and download the archive for your platform and architecture.
For a typical 64-bit Windows computer, choose the asset whose name ends in
`x86_64-pc-windows.zip`.

Do not install Magma by cloning the repository or by downloading GitHub's
automatically generated "Source code" archives. Those are source checkouts,
not release packages: executable files and required native or dynamic library
artifacts may be stripped or omitted. In particular, standard-library features
with native dependencies rely on the files distributed in the release archive.

Extract the complete archive to a permanent directory. For example:

```text
C:\Tools\Magma
```

Do not move only `Magma.exe` out of the extracted directory. By default the
compiler locates the standard library in the `std` directory beside itself, and
that tree contains platform implementations and bundled native libraries.

After extraction, the installation should resemble:

```text
C:\Tools\Magma\
|-- Magma.exe
|-- ADD_MAGMA_TO_PATH.bat
|-- VERSION.txt
|-- docs\
|-- samples\
`-- std\
```

Windows may mark downloaded archives and executables as originating from the
Internet. If Windows blocks execution, open the archive or `Magma.exe`
properties, select **Unblock**, apply the change, and extract again if needed.

## 2. Install LLVM Clang

Magma emits LLVM IR and invokes Clang to optimize it and produce object files
or executables. `Magma.exe` therefore requires a working `clang.exe`; the
compiler alone is not sufficient.

Download an official Windows LLVM installer from the
[LLVM release page](https://github.com/llvm/llvm-project/releases/latest). Run the
installer and install LLVM in its conventional location:

```text
C:\Program Files\LLVM
```

When the installer offers to add LLVM to `PATH`, select the option to add it
for the current user or all users. Open a new PowerShell or Command Prompt after
the installer completes, then verify it:

```powershell
clang --version
```

Magma searches for Clang in this order:

1. the executable named by `MAGMA_CLANG`;
2. `clang` on `PATH`;
3. locations named by `LLVM_HOME` or `LLVM_PATH`;
4. conventional LLVM, Visual Studio LLVM, and package-manager locations.

Consequently, either of the following configurations is sufficient.

### Option A: Add LLVM to `PATH`

Add this directory—not `clang.exe` itself—to the user or system `PATH`:

```text
C:\Program Files\LLVM\bin
```

Close and reopen terminals afterward. Confirm that Windows resolves the
expected compiler:

```powershell
where.exe clang
clang --version
```

### Option B: Point Magma at a specific Clang

If LLVM must not be placed on `PATH`, set `MAGMA_CLANG` to its executable:

```powershell
[Environment]::SetEnvironmentVariable(
    "MAGMA_CLANG",
    "C:\Program Files\LLVM\bin\clang.exe",
    "User"
)
```

Open a new terminal after setting it. A temporary setting for only the current
PowerShell session is also possible:

```powershell
$env:MAGMA_CLANG = "C:\Program Files\LLVM\bin\clang.exe"
```

Verify the Clang instance that Magma resolved:

```powershell
C:\Tools\Magma\Magma.exe --clang-version
```

If discovery fails, check that the path points to `clang.exe`, not LLVM's root
directory, and that `clang --version` runs without missing-DLL errors.

## 3. Add Magma to `PATH`

The release includes `ADD_MAGMA_TO_PATH.bat`. It validates that `Magma.exe` is
beside the script and adds that directory to the current user's `PATH` without
requiring administrator access.

Run the script from the extracted release directory, either by double-clicking
it or from a terminal:

```powershell
cd C:\Tools\Magma
.\ADD_MAGMA_TO_PATH.bat
```

The script is safe to run again: if the directory is already present, it does
not add a duplicate. Close every existing terminal and open a new one so it
receives the updated environment, then verify the installation:

```powershell
where.exe Magma.exe
Magma.exe --version
Magma.exe --clang-version
```

If `Magma.exe` is not found, ensure the release has not been moved since the
script was run. Run the script again from its new permanent location if it has.

## 4. Compile and run a program

Create a file named `hello.mg`:

```magma
mod main

use "std:io" io

pub main(args str[]) !void:
    io.printLn("Hello, World!")
..
```

From the directory containing the file, compile it to an executable:

```powershell
Magma.exe --emit exe --out hello.exe hello.mg
```

Executable output and optimization level 3 are the defaults, so the shorter
form is also valid:

```powershell
Magma.exe --out hello.exe hello.mg
```

Run the result:

```powershell
.\hello.exe
```

Arguments following the executable name are supplied to `main(args str[])`:

```powershell
.\hello.exe first second
```

### Other output formats

Use `--emit llvm` to inspect generated LLVM IR or `--emit object` to create an
object file without linking an executable:

```powershell
Magma.exe --emit llvm --out hello.ll hello.mg
Magma.exe --emit object --out hello.obj hello.mg
```

Select an optimization level with `-O0` through `-O3`:

```powershell
Magma.exe -O2 --out hello.exe hello.mg
```

The installed release automatically uses its adjacent `std` directory. The
`--std <directory>` option is intended for compiler or standard-library
development and normally should not be needed by release users.

For language syntax, continue with [Magma Syntax](SYNTAX.md). For every compiler
option and the language-server mode, see [Magma Compiler](COMPILER.md). Working
programs are available in the release's `samples` directory.

## Troubleshooting

- **`Magma.exe` is not recognized:** open a new terminal, run
  `where.exe Magma.exe`, and rerun `ADD_MAGMA_TO_PATH.bat` from the permanent
  installation directory.
- **Magma cannot find a usable Clang:** run `clang --version`, add LLVM's `bin`
  directory to `PATH`, or set `MAGMA_CLANG` to the full `clang.exe` path.
- **The standard library cannot be found:** keep `std` beside `Magma.exe` and
  preserve the release directory structure.
- **A native library or DLL is missing:** reinstall from the matching release
  archive rather than a repository clone or source archive, and do not remove
  files from `std/vendor`.
- **A newly set environment variable is ignored:** close and reopen the terminal
  because existing processes retain their old environment.
