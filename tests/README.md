# Compilation tests

`RUN_TESTS.bat` recursively compiles every `.mg` file below this directory.
Compilation success is expected by default.

Put an empty `.expect-failure` file in a directory when every `.mg` test in
that directory and its descendants is expected to be rejected by the compiler.
A test passes only when its actual compilation result matches that expectation.

