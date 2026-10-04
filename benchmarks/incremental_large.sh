#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 || $# -gt 2 ]]; then
    echo "usage: $0 <llvm-object-magma> [runs]" >&2
    exit 2
fi

compiler=$1
runs=${2:-5}
repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
std_root="$repo_root/std"
result_root=$(mktemp -d "${TMPDIR:-/tmp}/magma-incremental-large.XXXXXX")

cleanup() {
    rm -rf "$result_root"
}
trap cleanup EXIT

if [[ ! -x "$compiler" ]]; then
    echo "compiler path must be executable" >&2
    exit 2
fi
if [[ ! "$runs" =~ ^[1-9][0-9]*$ ]]; then
    echo "runs must be a positive integer" >&2
    exit 2
fi

compile() {
    local mode=$1
    local case_name=$2
    local run=$3
    local fixture="$result_root/fixture-${mode}/main.mg"
    local output="$result_root/${mode}-${case_name}-${run}.o"
    local metrics="$result_root/${mode}-${case_name}-${run}.metrics"
    local started_ns finished_ns
    local mode_args=()

    case "$mode" in
        textual-whole)
            mode_args=(--strategy textual)
            ;;
        incremental-bitcode)
            mode_args=(--cache-dir "$result_root/cache-${mode}")
            ;;
        *)
            echo "unknown benchmark mode: $mode" >&2
            exit 2
            ;;
    esac

    started_ns=$(date +%s%N)
    "$compiler" --std "$std_root" "${mode_args[@]}" --emit object --opt 3 --timings \
        --out "$output" "$fixture" 2>"$metrics"
    finished_ns=$(date +%s%N)
    printf 'mode=%s case=%s run=%d wall_ms=%d artifact_bytes=%s\n' \
        "$mode" "$case_name" "$run" "$(((finished_ns - started_ns) / 1000000))" \
        "$(stat -c %s "$output")"
    cat "$metrics"
}

echo "fixture=generated-8-modules-640-functions runs=$runs"
validation_failed=false
for mode in textual-whole incremental-bitcode; do
    "$repo_root/benchmarks/generate_incremental_large.sh" "$result_root/fixture-${mode}"
    leaf="$result_root/fixture-${mode}/workload_7.mg"
    for ((run = 1; run <= runs; run++)); do
        if [[ "$mode" != textual-whole ]]; then
            rm -rf "$result_root/cache-${mode}"
        fi
        compile "$mode" cold "$run"
        compile "$mode" unchanged-warm "$run"

        if grep -q 'result = result + 1000001' "$leaf"; then
            sed -i 's/result = result + 1000001/result = result + 1000003/' "$leaf"
        else
            sed -i 's/result = result + 1000003/result = result + 1000001/' "$leaf"
        fi
        compile "$mode" leaf-change-warm "$run"

        cold_object="$result_root/${mode}-cold-${run}.o"
        unchanged_object="$result_root/${mode}-unchanged-warm-${run}.o"
        changed_object="$result_root/${mode}-leaf-change-warm-${run}.o"
        if [[ "$mode" != textual-whole ]] && ! cmp -s "$cold_object" "$unchanged_object"; then
            echo "validation=FAIL mode=$mode run=$run unchanged object differs from cold object" >&2
            validation_failed=true
        fi
        if cmp -s "$cold_object" "$changed_object"; then
            echo "validation=FAIL mode=$mode run=$run leaf implementation change returned stale object" >&2
            validation_failed=true
        else
            echo "validation=PASS mode=$mode run=$run leaf implementation change altered object"
        fi
    done
done

if [[ "$validation_failed" == true ]]; then
    exit 1
fi
