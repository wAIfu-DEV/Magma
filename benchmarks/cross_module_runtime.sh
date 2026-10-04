#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 || $# -gt 2 ]]; then
    echo "usage: $0 <llvm-object-magma> [runs]" >&2
    exit 2
fi

compiler=$1
runs=${2:-7}
repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
std_root="$repo_root/std"
result_root=$(mktemp -d "${TMPDIR:-/tmp}/magma-cross-module-runtime.XXXXXX")

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

compile_mode() {
    local workload=$1
    local mode=$2
    local fixture="$result_root/fixture-${workload}/main.mg"
    local output="$result_root/${workload}-${mode}"
    local args=()
    case "$mode" in
        whole-program)
			args=(--strategy whole --incremental=false)
            ;;
		thinlto)
			args=(--strategy thinlto --jobs 12 --cache-dir "$result_root/cache-${workload}-thinlto")
			;;
    esac
    "$compiler" --std "$std_root" "${args[@]}" --emit exe --opt 3 --out "$output" "$fixture"
    printf 'workload=%s mode=%s executable_bytes=%s\n' "$workload" "$mode" "$(stat -c %s "$output")"
}

for workload in low medium heavy; do
    "$repo_root/benchmarks/generate_cross_module_runtime.sh" "$result_root/fixture-${workload}" "$workload"
    for mode in whole-program thinlto; do
        compile_mode "$workload" "$mode"
    done

    expected_status=
    for ((run = 1; run <= runs; run++)); do
		if ((run % 2 == 0)); then
			modes=(thinlto whole-program)
		else
			modes=(whole-program thinlto)
		fi
        for mode in "${modes[@]}"; do
            executable="$result_root/${workload}-${mode}"
            started_ns=$(date +%s%N)
            set +e
            "$executable"
            status=$?
            set -e
            finished_ns=$(date +%s%N)
            if [[ -z "$expected_status" ]]; then
                expected_status=$status
            elif [[ "$status" -ne "$expected_status" ]]; then
                echo "runtime result mismatch: workload=$workload mode=$mode run=$run status=$status expected=$expected_status" >&2
                exit 1
            fi
            printf 'workload=%s mode=%s run=%d wall_ns=%d status=%d\n' \
                "$workload" "$mode" "$run" "$((finished_ns - started_ns))" "$status"
        done
    done
done
