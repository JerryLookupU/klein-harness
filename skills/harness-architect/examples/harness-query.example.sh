#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -lt 2 ]; then
  echo "usage: $0 <overview|progress|current|blueprint|task|feedback> <ROOT> [TASK_ID] [--text]" >&2
  exit 1
fi

VIEW="$1"
ROOT="$2"
TASK_ID="${3:-}"
FORMAT_FLAG="${4:-}"
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

if [ "$TASK_ID" = "--text" ]; then
  TASK_ID=""
  FORMAT_FLAG="--text"
fi

if [ "$FORMAT_FLAG" = "--text" ]; then
  if [ -n "$TASK_ID" ]; then
    python3 "$PYTHON_QUERY" --root "$ROOT" --view "$VIEW" --task-id "$TASK_ID" --format text
  else
    python3 "$PYTHON_QUERY" --root "$ROOT" --view "$VIEW" --format text
  fi
else
  if [ -n "$TASK_ID" ]; then
    python3 "$PYTHON_QUERY" --root "$ROOT" --view "$VIEW" --task-id "$TASK_ID" --format json
  else
    python3 "$PYTHON_QUERY" --root "$ROOT" --view "$VIEW" --format json
  fi
fi
