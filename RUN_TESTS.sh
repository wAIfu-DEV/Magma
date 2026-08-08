#!/usr/bin/env bash

set -u

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
TEST_ROOT="$ROOT/tests"
STD_TEST_ROOT="$ROOT/std/tests"
case "${OSTYPE:-}" in
    cygwin*|msys*|win32*) COMPILER="$ROOT/Magma.exe"; EXECUTABLE_SUFFIX=".exe" ;;
    *)                    COMPILER="$ROOT/Magma";     EXECUTABLE_SUFFIX="" ;;
esac
cd "$ROOT" || exit 1

TEMP_ROOT="${TMPDIR:-/tmp}"
WORK_DIR="$(mktemp -d "$TEMP_ROOT/magma-tests.XXXXXX")" || exit 1
LOG_FILE="$WORK_DIR/compiler.log"
OUTPUT_FILE="$WORK_DIR/output.ll"
trap 'rm -rf -- "$WORK_DIR"' EXIT INT TERM

total=0
passed=0
failed=0

if [[ ! -x "$COMPILER" ]]; then
    echo "[FAIL] Magma compiler not found or not executable: $COMPILER" >&2
    echo "Build the compiler before running the tests." >&2
    exit 1
fi

for directory in "$TEST_ROOT" "$STD_TEST_ROOT"; do
    if [[ ! -d "$directory" ]]; then
        echo "[FAIL] Test directory not found: $directory" >&2
        exit 1
    fi
done

shopt -s nullglob
for module in "$ROOT"/std/*.mg; do
    [[ "$(basename -- "$module")" == "raylib.mg" ]] && continue
    test_file="$STD_TEST_ROOT/$(basename -- "$module")"
    if [[ ! -f "$test_file" ]]; then
        echo "[FAIL] Missing standard library test: $test_file" >&2
        exit 1
    fi
done
for test_file in "$STD_TEST_ROOT"/*.mg; do
    module="$ROOT/std/$(basename -- "$test_file")"
    if [[ ! -f "$module" ]]; then
        echo "[FAIL] Standard library test has no matching module: $test_file" >&2
        exit 1
    fi
done

read -r -p "Display compilation output? [y/N]: " show_output || true
[[ "${show_output:-n}" =~ ^[Yy]$ ]] || show_output=n

now_ms() {
    local value
    value="$(date +%s%3N 2>/dev/null)"
    if [[ "$value" =~ ^[0-9]+$ ]]; then
        printf '%s\n' "$value"
    else
        printf '%s000\n' "$(date +%s)"
    fi
}

run_one() {
    local test_file="$1" suite_root="$2" run_assertions="$3"
    local expect=success check_dir compile_start compile_end compile_time compile_exit
    local run_start run_end run_time run_exit executable_file

    ((total += 1))
    check_dir="$(dirname -- "$test_file")"
    while :; do
        if [[ -e "$check_dir/.expect-failure" ]]; then
            expect=failure
            break
        fi
        [[ "$check_dir" == "$suite_root" ]] && break
        check_dir="$(dirname -- "$check_dir")"
    done

    rm -f -- "$OUTPUT_FILE" "$LOG_FILE"
    executable_file="$WORK_DIR/test-$total$EXECUTABLE_SUFFIX"
    compile_start="$(now_ms)"
    if [[ "$run_assertions" == y ]]; then
        "$COMPILER" --std "$ROOT/std" --emit exe --out "$executable_file" "$test_file" >"$LOG_FILE" 2>&1
    else
        "$COMPILER" --std "$ROOT/std" --emit llvm --out "$OUTPUT_FILE" "$test_file" >"$LOG_FILE" 2>&1
    fi
    compile_exit=$?
    compile_end="$(now_ms)"
    compile_time=$((compile_end - compile_start))

    if [[ "$show_output" =~ ^[Yy]$ ]]; then
        echo
        echo "----- Compiler output: $test_file -----"
        cat -- "$LOG_FILE"
        echo "----- End compiler output -----"
    fi

    if [[ "$expect" == failure ]]; then
        if ((compile_exit != 0)); then
            if grep -Eqi 'panic:|goroutine |uncaught fatal error|Clang failed|fatal error in file|internal compiler error' "$LOG_FILE"; then
                ((failed += 1))
                echo "[FAILURE] $test_file ${compile_time} ms - rejected by a compiler crash or backend failure."
                [[ "$show_output" =~ ^[Yy]$ ]] || cat -- "$LOG_FILE"
            else
                ((passed += 1))
                echo "[PASS] $test_file ${compile_time} ms (rejected as expected)"
            fi
        else
            ((failed += 1))
            echo "[FAILURE] $test_file ${compile_time} ms - compiled successfully; expected rejection."
        fi
        return
    fi

    if ((compile_exit != 0)); then
        ((failed += 1))
        echo "[FAILURE] $test_file ${compile_time} ms - failed to compile; expected success."
        [[ "$show_output" =~ ^[Yy]$ ]] || cat -- "$LOG_FILE"
        echo
        return
    fi

    if [[ "$run_assertions" != y ]]; then
        ((passed += 1))
        echo "[PASS] $test_file ${compile_time} ms"
        return
    fi

    run_start="$(now_ms)"
    "$executable_file" >>"$LOG_FILE" 2>&1
    run_exit=$?
    run_end="$(now_ms)"
    run_time=$((run_end - run_start))
    rm -f -- "$executable_file"
    if ((run_exit == 0)); then
        ((passed += 1))
        echo "[PASS] $test_file ${compile_time} ms compile, ${run_time} ms assertions"
    else
        ((failed += 1))
        echo "[FAILURE] $test_file ${run_time} ms - assertions failed with exit code $run_exit."
        cat -- "$LOG_FILE"
        echo
    fi
}

echo
echo "Running Magma compilation tests..."
while IFS= read -r -d '' test_file; do
    run_one "$test_file" "$TEST_ROOT" n
done < <(find "$TEST_ROOT" -type f -name '*.mg' -print0)

echo
echo "Running standard library compilation and assertion tests..."
while IFS= read -r -d '' test_file; do
    run_one "$test_file" "$STD_TEST_ROOT" y
done < <(find "$STD_TEST_ROOT" -type f -name '*.mg' -print0)

echo
echo "Results: $passed passed, $failed failed, $total total."
if ((total == 0)); then
    echo "[FAIL] No .mg test files were found." >&2
    exit 1
fi
((failed == 0))
