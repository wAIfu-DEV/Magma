#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
    echo "usage: $0 <output-directory>" >&2
    exit 2
fi

output_dir=$1
modules=8
functions_per_module=80
mkdir -p "$output_dir"

for ((module = 0; module < modules; module++)); do
    file="$output_dir/workload_${module}.mg"
    printf 'mod workload_%d\n\n' "$module" >"$file"
    for ((fn = 0; fn < functions_per_module; fn++)); do
        multiplier=$((fn % 7 + 3))
        increment=$((module * functions_per_module + fn + 1))
        printf 'step%d(value i32) i32:\n' "$fn" >>"$file"
        printf '    mixed := value * %d + %d\n' "$multiplier" "$increment" >>"$file"
        printf '    if mixed < value:\n' >>"$file"
        printf '        ret mixed + %d\n' "$((increment % 13 + 1))" >>"$file"
        printf '    ..\n' >>"$file"
        printf '    ret mixed - %d\n' "$((increment % 11 + 1))" >>"$file"
        printf '..\n\n' >>"$file"
    done
    printf 'pub run(value i32) i32:\n' >>"$file"
    printf '    result := value\n' >>"$file"
    for ((fn = 0; fn < functions_per_module; fn++)); do
        printf '    result = step%d(result)\n' "$fn" >>"$file"
    done
    if [[ "$module" -eq $((modules - 1)) ]]; then
        printf '    result = result + 1000001\n' >>"$file"
    fi
    printf '    ret result\n' >>"$file"
    printf '..\n' >>"$file"
done

main_file="$output_dir/main.mg"
printf 'mod main\n\n' >"$main_file"
printf 'ext ext_getpid getpid() i32\n' >>"$main_file"
printf 'ext ext_exit exit(status i32) void\n\n' >>"$main_file"
for ((module = 0; module < modules; module++)); do
    printf 'use "workload_%d.mg" as workload%d\n' "$module" "$module" >>"$main_file"
done
printf '\npub main() void:\n' >>"$main_file"
printf '    result := ext_getpid()\n' >>"$main_file"
for ((module = 0; module < modules; module++)); do
    printf '    result = workload%d.run(result)\n' "$module" >>"$main_file"
done
printf '    ext_exit(result)\n' >>"$main_file"
printf '..\n' >>"$main_file"
