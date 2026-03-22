#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -lt 4 ]; then
  echo "usage: $0 <TASK_ID> <ROOT> <ROLE> <STAGE>" >&2
  echo "example: $0 T-002 . worker start" >&2
  exit 1
fi

TASK_ID="$1"
ROOT="$2"
ROLE="$3"
STAGE="$4"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PYTHON_RENDER=""

if [ -f "$SCRIPT_DIR/../scripts/render-prompt.py" ]; then
  PYTHON_RENDER="$SCRIPT_DIR/../scripts/render-prompt.py"
elif [ -f "$SCRIPT_DIR/render-prompt.example.py" ]; then
  PYTHON_RENDER="$SCRIPT_DIR/render-prompt.example.py"
else
  echo "render-prompt script not found" >&2
  exit 1
fi

python3 "$PYTHON_RENDER" \
  --root "$ROOT" \
  --task-id "$TASK_ID" \
  --role "$ROLE" \
  --stage "$STAGE"
