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
PYTHON_VERIFY=""

if [ -f "$SCRIPT_DIR/../scripts/verify-task.py" ]; then
  PYTHON_VERIFY="$SCRIPT_DIR/../scripts/verify-task.py"
elif [ -f "$SCRIPT_DIR/verify-task.example.py" ]; then
  PYTHON_VERIFY="$SCRIPT_DIR/verify-task.example.py"
else
  echo "verify-task script not found" >&2
  exit 1
fi

if [ "$WRITE_BACK_FLAG" = "--write-back" ]; then
  python3 "$PYTHON_VERIFY" --root "$ROOT" --task-id "$TASK_ID" --write-back
else
  python3 "$PYTHON_VERIFY" --root "$ROOT" --task-id "$TASK_ID"
fi
