#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 2 || $# -gt 3 ]]; then
    echo "usage: $0 <textual-magma> <llvm-object-magma> [runs]" >&2
    exit 2
fi

textual_compiler=$1
object_compiler=$2
runs=${3:-5}
repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
fixture="$repo_root/tests/incremental_baseline/main.mg"
std_root="$repo_root/std"
result_root=$(mktemp -d "${TMPDIR:-/tmp}/magma-incremental-baseline.XXXXXX")

cleanup() {
    rm -rf "$result_root"
}
trap cleanup EXIT

if [[ ! -x "$textual_compiler" || ! -x "$object_compiler" ]]; then
    echo "both compiler paths must be executable" >&2
    exit 2
fi
if [[ ! "$runs" =~ ^[1-9][0-9]*$ ]]; then
    echo "runs must be a positive integer" >&2
    exit 2
fi
gnu_time=
if [[ -x /usr/bin/time ]]; then
    gnu_time=/usr/bin/time
elif [[ -x /bin/time ]]; then
    gnu_time=/bin/time
else
    echo "warning: GNU time not found; peak RSS and CPU time will be unavailable" >&2
fi

run_case() {
    local name=$1
    local compiler=$2
    local backend=$3
    local optimization=$4
    local incremental=${5:-false}
    local run output metrics
	local extra_args=()
	if [[ "$incremental" == true ]]; then
		extra_args=(--incremental --cache-dir "$result_root/cache")
	fi

    for ((run = 1; run <= runs; run++)); do
        output="$result_root/${name}-${run}"
        metrics="$result_root/${name}-${run}.metrics"
        if [[ -n "$gnu_time" ]]; then
            "$gnu_time" -f 'wall_seconds=%e cpu_user_seconds=%U cpu_system_seconds=%S peak_rss_kb=%M' \
                -o "$metrics" "$compiler" --std "$std_root" --strategy "$backend" \
                --emit exe --opt "$optimization" --timings "${extra_args[@]}" --out "$output" "$fixture" \
                2>>"$metrics"
        else
            local started_ns finished_ns
            started_ns=$(date +%s%N)
            "$compiler" --std "$std_root" --strategy "$backend" --emit exe \
                --opt "$optimization" --timings "${extra_args[@]}" --out "$output" "$fixture" 2>>"$metrics"
            finished_ns=$(date +%s%N)
            printf 'wall_nanoseconds=%s peak_rss_kb=unavailable cpu_time=unavailable\n' \
                "$((finished_ns - started_ns))" >>"$metrics"
        fi
        printf 'case=%s run=%d artifact_bytes=%s\n' "$name" "$run" "$(stat -c %s "$output")"
        cat "$metrics"
        "$output"
    done
}

echo "fixture=$fixture runs=$runs"
run_case textual-o0 "$textual_compiler" textual 0
run_case textual-o3 "$textual_compiler" textual 3
run_case object-o0 "$object_compiler" object 0
run_case object-o3 "$object_compiler" object 3
run_case incremental-cold-warm-o3 "$object_compiler" object 3 true
