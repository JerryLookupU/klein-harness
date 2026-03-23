#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -lt 1 ]; then
  cat <<'EOF' >&2
usage: harness-runner <tick|run|recover|finalize|attach|list|daemon|daemon-stop|daemon-status> [args...]

commands:
  tick <ROOT> [--trigger shell] [--dispatch-mode tmux|print]
  run <TASK_ID> <ROOT> [--trigger shell] [--dispatch-mode tmux|print]
  recover <TASK_ID> <ROOT> [--trigger shell] [--dispatch-mode tmux|print]
  finalize <ROOT> <TASK_ID> [--tmux-session NAME] [--runner-status N]
  attach <TASK_ID> <ROOT>
  list <ROOT>
  daemon <ROOT> [--interval N] [--dispatch-mode tmux|print] [--replace]
  daemon-stop <ROOT>
  daemon-status <ROOT>
EOF
  exit 1
fi

COMMAND="$1"
shift
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PYTHON_RUNNER=""

if [ -f "$SCRIPT_DIR/../scripts/runner.py" ]; then
  PYTHON_RUNNER="$SCRIPT_DIR/../scripts/runner.py"
elif [ -f "$SCRIPT_DIR/runner.example.py" ]; then
  PYTHON_RUNNER="$SCRIPT_DIR/runner.example.py"
else
  echo "runner script not found" >&2
  exit 1
fi

attach_task() {
  if [ "$#" -lt 2 ]; then
    echo "usage: harness-runner attach <TASK_ID> <ROOT>" >&2
    exit 1
  fi

  local task_id="$1"
  local root="$2"
  local heartbeat_path="$root/.harness/state/runner-heartbeats.json"

  if [ ! -f "$heartbeat_path" ]; then
    echo "runner heartbeats not found: $heartbeat_path" >&2
    exit 1
  fi

  local session_name
  session_name="$({ python3 - "$heartbeat_path" "$task_id" <<'PY'
import json
import sys
from pathlib import Path

path = Path(sys.argv[1])
task_id = sys.argv[2]
data = json.loads(path.read_text())
entry = (data.get("entries") or {}).get(task_id) or {}
print(entry.get("tmuxSession") or "")
PY
} )"

  if [ -z "$session_name" ]; then
    echo "no tmux session recorded for task: $task_id" >&2
    exit 1
  fi

  tmux attach -t "$session_name"
}

case "$COMMAND" in
  attach)
    attach_task "$@"
    ;;
  tick|run|recover|finalize|list|daemon|daemon-stop|daemon-status)
    python3 "$PYTHON_RUNNER" "$COMMAND" "$@"
    ;;
  *)
    echo "unknown command: $COMMAND" >&2
    exit 1
    ;;
esac
