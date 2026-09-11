#!/usr/bin/env bash

set -u

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT" || exit 1

echo "Building Magma Compiler..."
if go build -tags "llvm_object llvm22"; then
    echo
    echo "Compilation success."
else
    status=$?
    echo >&2
    echo "Compilation failed." >&2
    exit "$status"
fi
