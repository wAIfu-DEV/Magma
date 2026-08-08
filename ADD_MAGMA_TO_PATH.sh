#!/usr/bin/env bash

set -u

MAGMA_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
if [[ -x "$MAGMA_DIR/Magma" ]]; then
    MAGMA_EXECUTABLE="$MAGMA_DIR/Magma"
elif [[ -f "$MAGMA_DIR/Magma.exe" ]]; then
    MAGMA_EXECUTABLE="$MAGMA_DIR/Magma.exe"
else
    echo "Magma executable was not found in '$MAGMA_DIR'." >&2
    exit 1
fi

case ":$PATH:" in
    *":$MAGMA_DIR:"*)
        echo "Already on PATH in this shell: $MAGMA_DIR"
        ;;
    *)
        export PATH="$MAGMA_DIR:$PATH"
        ;;
esac

case "${SHELL:-}" in
    */zsh) profile="$HOME/.zshrc" ;;
    *)     profile="$HOME/.bashrc" ;;
esac

path_line="export PATH=\"$MAGMA_DIR:\$PATH\""
if [[ -f "$profile" ]] && grep -Fqx -- "$path_line" "$profile"; then
    echo "Already configured in $profile: $MAGMA_DIR"
else
    printf '\n# Magma compiler\n%s\n' "$path_line" >> "$profile" || {
        echo "Failed to update '$profile'." >&2
        exit 1
    }
    echo "Added to PATH in $profile: $MAGMA_DIR"
fi

echo
echo "Run 'source \"$profile\"' or open a new terminal, then run: $(basename -- "$MAGMA_EXECUTABLE")"
