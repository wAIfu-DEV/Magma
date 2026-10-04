#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 || $# -gt 2 ]]; then
    echo "usage: $0 <output-directory> [low|medium|heavy]" >&2
    exit 2
fi

output_dir=$1
workload=${2:-medium}
iterations=10000000
kernel=${MAGMA_CROSS_MODULE_KERNEL:-branch}
if [[ "$kernel" != branch && "$kernel" != trivial ]]; then
    echo "unknown MAGMA_CROSS_MODULE_KERNEL: $kernel" >&2
    exit 2
fi
case "$workload" in
    low) modules=2 ;;
    medium) modules=8 ;;
    heavy) modules=16 ;;
    *)
    echo "unknown workload: $workload" >&2
    exit 2
    ;;
esac
mkdir -p "$output_dir"

for ((module = 0; module < modules; module++)); do
    multiplier=$((module % 5 + 3))
    increment=$((module * 17 + 11))
    file="$output_dir/stage_${module}.mg"
    printf 'mod stage_%d\n\n' "$module" >"$file"
    printf 'pub step(value i32) i32:\n' >>"$file"
    if [[ "$kernel" == trivial ]]; then
        printf '    ret value + %d\n' "$increment" >>"$file"
    else
        printf '    mixed := value * %d + %d\n' "$multiplier" "$increment" >>"$file"
        printf '    if mixed < value:\n' >>"$file"
        printf '        ret mixed + %d\n' "$((module + 3))" >>"$file"
        printf '    ..\n' >>"$file"
        printf '    ret mixed - %d\n' "$((module + 1))" >>"$file"
    fi
    printf '..\n' >>"$file"
done

main_file="$output_dir/main.mg"
printf 'mod main\n\n' >"$main_file"
printf 'ext ext_exit exit(status i32) void\n\n' >>"$main_file"
printf 'ext ext_getppid getppid() i32\n\n' >>"$main_file"
for ((module = 0; module < modules; module++)); do
    printf 'use "stage_%d.mg" as stage%d\n' "$module" "$module" >>"$main_file"
done
printf '\npub main() void:\n' >>"$main_file"
printf '    value i32 = ext_getppid()\n' >>"$main_file"
printf '    iteration i32 = 0\n' >>"$main_file"
printf '    loop iteration < %d:\n' "$iterations" >>"$main_file"
for ((module = 0; module < modules; module++)); do
    printf '        value = stage%d.step(value)\n' "$module" >>"$main_file"
done
printf '        iteration = iteration + 1\n' >>"$main_file"
printf '    ..\n' >>"$main_file"
printf '    ext_exit(value)\n' >>"$main_file"
printf '..\n' >>"$main_file"
