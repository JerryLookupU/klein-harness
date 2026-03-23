#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -lt 1 ]; then
  cat <<'EOF' >&2
usage: harness-submit <ROOT> --goal <TEXT> [options...]

Single public write path for initial work, appended work, duplicate/context submissions, and inspection intents.
`--kind` is optional and treated as a hint.
`--context` may be repeated.
EOF
  exit 1
fi

ROOT="$1"
shift
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PYTHON_REQUEST=""

if [ -f "$SCRIPT_DIR/../scripts/request.py" ]; then
  PYTHON_REQUEST="$SCRIPT_DIR/../scripts/request.py"
elif [ -f "$SCRIPT_DIR/request.example.py" ]; then
  PYTHON_REQUEST="$SCRIPT_DIR/request.example.py"
else
  echo "request script not found" >&2
  exit 1
fi

python3 "$PYTHON_REQUEST" submit --root "$ROOT" "$@"
