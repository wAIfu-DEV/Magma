# Magma Compiler

## Usage

```text
magma [options] <input-file>
```

With no output option, the compiler builds an executable named for the selected
platform. Compilation performs parsing, generic specialization, name linking,
type and lowering validation, warning-only ownership analysis, LLVM lowering,
and native emission.

## Output

- `--emit`, `-e` selects `llvm`, `object`, or `exe`. The aliases `ll`, `obj`,
  `o`, `executable`, `binary`, and `bin` are also accepted.
- `--out`, `-o` selects the output path.
- `--opt`, `-O` selects LLVM optimization level 0 through 3. Compact forms
  such as `-O2` are accepted; the default is 3.
- `--target` selects a Clang target triple or architecture. With no value, the
  resolved Clang installation's native target is used.

Executable emission resolves declarations made with `link` and copies files
declared with `bundle` beside the completed executable. LLVM and object
emission do not link libraries or copy bundles.

## Compiler environment and diagnostics

- `--std <directory>` overrides the standard-library directory. Otherwise the
  compiler uses the `std` directory beside its executable.
- `--debug` prints compiler diagnostics such as the resolved target and input.
- `--error-trace-slots <n>` sets runtime propagation-trace capacity. It must be
  a power of two from 1 through 1024 and defaults to 1024.
- `--safety-warnings` downgrades fatal ownership-safety diagnostics to warnings
  for migration. It does not disable analysis or change `move` semantics.
- `--version`, `-v` prints the Magma version.
- `--clang-version`, `-cv` prints the resolved Clang version and path.

Information commands do not accept an input file.

## Language server

`--lsp` runs the Magma language server over standard input and output. It
provides diagnostics (including compiler warnings), completion, hover
documentation, definition lookup, import-path completion, safety quick fixes,
and semantic highlighting for `move`, `bounded`, and `unsafe`. It uses the same
standard-library discovery and `--std` override as normal compilation.

The LSP defaults to fatal safety enforcement. Starting it with
`--safety-warnings --lsp`, setting `initializationOptions.safetyWarnings`, or
sending `workspace/didChangeConfiguration` with `settings.safetyWarnings`
selects warning mode. Both policies publish identical diagnostic codes,
locations, messages, and related locations; only severity changes.
