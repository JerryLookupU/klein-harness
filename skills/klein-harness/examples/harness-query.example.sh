#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -lt 2 ]; then
  echo "usage: $0 <overview|progress|current|blueprint|task|feedback|logs|log> <ROOT> [args...] [--text]" >&2
  exit 1
fi

VIEW="$1"
ROOT="$2"
shift 2
ARGS=("$@")
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PYTHON_QUERY=""

if [ -f "$SCRIPT_DIR/../scripts/query.py" ]; then
  PYTHON_QUERY="$SCRIPT_DIR/../scripts/query.py"
elif [ -f "$SCRIPT_DIR/query-harness.example.py" ]; then
  PYTHON_QUERY="$SCRIPT_DIR/query-harness.example.py"
else
  echo "query script not found" >&2
  exit 1
fi

FORMAT="json"
PASSTHRU=()
POSITIONAL_TASK=""

for arg in "${ARGS[@]}"; do
  if [ "$arg" = "--text" ]; then
    FORMAT="text"
    continue
  fi
  if [ -z "$POSITIONAL_TASK" ] && [[ "$arg" != --* ]] && [ "$VIEW" != "overview" ] && [ "$VIEW" != "progress" ] && [ "$VIEW" != "current" ] && [ "$VIEW" != "blueprint" ] && [ "$VIEW" != "logs" ]; then
    POSITIONAL_TASK="$arg"
    continue
  fi
  PASSTHRU+=("$arg")
done

if [ -n "$POSITIONAL_TASK" ]; then
  PASSTHRU=(--task-id "$POSITIONAL_TASK" "${PASSTHRU[@]}")
fi

python3 "$PYTHON_QUERY" --root "$ROOT" --view "$VIEW" --format "$FORMAT" "${PASSTHRU[@]}"
