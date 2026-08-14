#!/usr/bin/env bash

set -u

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
GOCACHE="$ROOT/.gocache"
export GOCACHE
cd "$ROOT" || exit 1

if ! command -v go >/dev/null 2>&1; then
    echo "Go was not found on PATH." >&2
    exit 1
fi
if ! command -v zip >/dev/null 2>&1; then
    echo "zip was not found on PATH." >&2
    exit 1
fi

echo "Building the release compiler..."
if ! go build -trimpath -o "$ROOT/Magma" .; then
    echo "Compiler build failed." >&2
    exit 1
fi

echo "Running the release test suite..."
if ! "$ROOT/RUN_TESTS.sh"; then
    echo "Test suite failed. Release aborted." >&2
    exit 1
fi

echo "Cross-compiling Windows amd64 release..."
if ! GOOS=windows GOARCH=amd64 go build -trimpath -o "$ROOT/Magma.exe" .; then
    echo "Windows amd64 build failed." >&2
    exit 1
fi

echo "Cross-compiling Linux amd64 release..."
if ! GOOS=linux GOARCH=amd64 go build -trimpath -o "$ROOT/Magma" .; then
    echo "Linux amd64 build failed." >&2
    exit 1
fi

current_version="$(tr -d '[:space:]' < "$ROOT/VERSION.txt")"
if [[ ! "$current_version" =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
    echo "VERSION.txt must contain a semantic version such as 1.2.3" >&2
    exit 1
fi
version="${BASH_REMATCH[1]}.${BASH_REMATCH[2]}.$((10#${BASH_REMATCH[3]} + 1))"
printf '%s\n' "$version" > "$ROOT/VERSION.txt"
echo "Releasing Magma $version..."

stage_root="$(mktemp -d "${TMPDIR:-/tmp}/magma-release.XXXXXX")" || exit 1
trap 'rm -rf -- "$stage_root"' EXIT INT TERM

mapfile -t ignore_patterns < <(sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//' -e '/^$/d' -e '/^#/d' "$ROOT/RELEASE_IGNORE.txt")

create_archive() {
    local binary="$1" platform="$2" archive stage source relative pattern ignored
    archive="magma-$version-$platform.zip"
    stage="$stage_root/$platform"
    mkdir -p -- "$stage"

    while IFS= read -r -d '' source; do
        relative="${source#"$ROOT"/}"
        ignored=false
        [[ "$relative" == Magma || "$relative" == Magma.exe ]] && ignored=true
        for pattern in "${ignore_patterns[@]}"; do
            if [[ "$relative" == $pattern ]]; then
                ignored=true
                break
            fi
        done
        if [[ "$ignored" == false ]]; then
            mkdir -p -- "$stage/$(dirname -- "$relative")"
            cp -p -- "$source" "$stage/$relative"
        fi
    done < <(find "$ROOT" -type f -print0)

    cp -p -- "$ROOT/$binary" "$stage/$binary"
    if ! (cd "$stage" && zip -q -r "$ROOT/$archive" .); then
        echo "Archive creation failed for $archive." >&2
        return 1
    fi
    echo "Created $archive"
}

create_archive Magma.exe x86_64-pc-windows || exit 1
create_archive Magma x86_64-unknown-linux || exit 1
