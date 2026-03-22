#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -lt 1 ]; then
  echo "usage: $0 <TASK_ID> [ROOT] [--write-back]" >&2
  exit 1
fi

TASK_ID="$1"
ROOT="${2:-$(pwd)}"
WRITE_BACK_FLAG="${3:-}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PYTHON_DIFF=""

if [ -f "$SCRIPT_DIR/../scripts/diff-summary.py" ]; then
  PYTHON_DIFF="$SCRIPT_DIR/../scripts/diff-summary.py"
elif [ -f "$SCRIPT_DIR/diff-summary.example.py" ]; then
  PYTHON_DIFF="$SCRIPT_DIR/diff-summary.example.py"
else
  echo "diff-summary script not found" >&2
  exit 1
fi

if [ "$WRITE_BACK_FLAG" = "--write-back" ]; then
  python3 "$PYTHON_DIFF" --root "$ROOT" --task-id "$TASK_ID" --write-back
else
  python3 "$PYTHON_DIFF" --root "$ROOT" --task-id "$TASK_ID"
fi
