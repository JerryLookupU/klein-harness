#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PYTHON_SCRIPT=""

if [ -f "$SCRIPT_DIR/../scripts/log-search.py" ]; then
  PYTHON_SCRIPT="$SCRIPT_DIR/../scripts/log-search.py"
elif [ -f "$SCRIPT_DIR/log-search.example.py" ]; then
  PYTHON_SCRIPT="$SCRIPT_DIR/log-search.example.py"
else
  echo "log search script not found" >&2
  exit 1
fi

if [ "$#" -lt 1 ]; then
  echo "usage: $0 <ROOT> [filters...] [--json]" >&2
  exit 1
fi

ROOT="$1"
shift
FORMAT="text"
PASSTHRU=()

for arg in "$@"; do
  if [ "$arg" = "--json" ]; then
    FORMAT="json"
    continue
  fi
  PASSTHRU+=("$arg")
done

python3 "$PYTHON_SCRIPT" --root "$ROOT" --format "$FORMAT" "${PASSTHRU[@]}"
