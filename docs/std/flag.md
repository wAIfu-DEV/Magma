# `std/flag`

`Parser` declares typed command-line options and parses an already tokenized
argument slice. It never invokes a shell or performs quote expansion.

```magma
parser := try flag.new(a, "build")
defer parser.free()

verbose bool = false
jobs u64 = 1
try parser.boolean("verbose", 118, addrof verbose, "verbose output")
try parser.unsigned("jobs", 106, addrof jobs, "worker count")

result := try parser.parse(command.values())
defer result.free()
positionals := result.positionals()
```

Long options accept `--name=value` or `--name value`; short valued options use
`-n value` or `-nvalue`. Boolean short options may be clustered as `-vq`.
`--` ends option parsing. Scalar duplicates, unknown options,
missing values, and malformed integers return `invalidArgument` (numeric
overflow remains `wouldOverflow`). `strings` appends repeated borrowed values
to a caller-owned `array.Array[str]`. Parser definitions and parsed strings are
borrowed; `Result` owns only its positional array storage.

`writeUsage(writer)` emits a compact option listing. Short names are supplied
as their ASCII byte value because Magma has no character literal type.
`unsigneds` and `integers` append repeated numeric options. `usage()` returns
the generated text as an allocator-owned string.
