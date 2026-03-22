#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -lt 1 ]; then
  echo "usage: $0 <TASK_ID> [ROOT] [--create]" >&2
  exit 1
fi

TASK_ID="$1"
ROOT="${2:-$(pwd)}"
CREATE_FLAG="${3:-}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PYTHON_PREPARE=""

if [ -f "$SCRIPT_DIR/../scripts/prepare-worktree.py" ]; then
  PYTHON_PREPARE="$SCRIPT_DIR/../scripts/prepare-worktree.py"
elif [ -f "$SCRIPT_DIR/prepare-worktree.example.py" ]; then
  PYTHON_PREPARE="$SCRIPT_DIR/prepare-worktree.example.py"
else
  echo "prepare-worktree script not found" >&2
  exit 1
fi

if [ "$CREATE_FLAG" = "--create" ]; then
  python3 "$PYTHON_PREPARE" --root "$ROOT" --task-id "$TASK_ID" --create
else
  python3 "$PYTHON_PREPARE" --root "$ROOT" --task-id "$TASK_ID"
fi
