#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -lt 1 ]; then
  echo "usage: $0 <PROJECT_ROOT>" >&2
  exit 1
fi

ROOT="$(cd "$1" && pwd)"
HARNESS_DIR="$ROOT/.harness"
BIN_DIR="$HARNESS_DIR/bin"
SCRIPTS_DIR="$HARNESS_DIR/scripts"
MANIFEST="$HARNESS_DIR/tooling-manifest.json"
EXAMPLES_DIR="$(cd "$(dirname "$0")" && pwd)"

mkdir -p "$BIN_DIR" "$SCRIPTS_DIR" "$HARNESS_DIR/state"

install_file() {
  local src="$1"
  local dst="$2"
  cp "$EXAMPLES_DIR/$src" "$dst"
}

install_file "harness-query.example.sh" "$BIN_DIR/harness-query"
install_file "harness-tasks.example.sh" "$BIN_DIR/harness-tasks"
install_file "harness-task.example.sh" "$BIN_DIR/harness-task"
install_file "harness-control.example.sh" "$BIN_DIR/harness-control"
install_file "harness-dashboard.example.sh" "$BIN_DIR/harness-dashboard"
install_file "query-harness.example.py" "$SCRIPTS_DIR/query.py"
install_file "control.example.py" "$SCRIPTS_DIR/control.py"
install_file "refresh-state.example.py" "$SCRIPTS_DIR/refresh-state.py"

chmod +x "$BIN_DIR/harness-query" "$BIN_DIR/harness-tasks" "$BIN_DIR/harness-task" "$BIN_DIR/harness-control" "$BIN_DIR/harness-dashboard"

cat > "$MANIFEST" <<'JSON'
{
  "schemaVersion": "1.0",
  "generator": "klein-harness",
  "generatedAt": "INSTALL_TIME",
  "installed": [
    {
      "name": "harness-query",
      "target": ".harness/bin/harness-query",
      "source": "examples/harness-query.example.sh"
    },
    {
      "name": "harness-tasks",
      "target": ".harness/bin/harness-tasks",
      "source": "examples/harness-tasks.example.sh"
    },
    {
      "name": "harness-task",
      "target": ".harness/bin/harness-task",
      "source": "examples/harness-task.example.sh"
    },
    {
      "name": "harness-control",
      "target": ".harness/bin/harness-control",
      "source": "examples/harness-control.example.sh"
    },
    {
      "name": "harness-dashboard",
      "target": ".harness/bin/harness-dashboard",
      "source": "examples/harness-dashboard.example.sh"
    },
    {
      "name": "query.py",
      "target": ".harness/scripts/query.py",
      "source": "examples/query-harness.example.py"
    },
    {
      "name": "control.py",
      "target": ".harness/scripts/control.py",
      "source": "examples/control.example.py"
    },
    {
      "name": "refresh-state.py",
      "target": ".harness/scripts/refresh-state.py",
      "source": "examples/refresh-state.example.py"
    }
  ]
}
JSON

python3 - <<'PY' "$MANIFEST"
import json, sys
from datetime import datetime, timezone
path = sys.argv[1]
data = json.load(open(path))
data["generatedAt"] = datetime.now(timezone.utc).astimezone().isoformat(timespec="seconds")
json.dump(data, open(path, "w"), ensure_ascii=False, indent=2)
open(path, "a").write("\n")
PY

echo "installed harness tools into $HARNESS_DIR"
echo "primary local commands:"
echo "  .harness/bin/harness-submit"
echo "  .harness/bin/harness-tasks"
echo "  .harness/bin/harness-task"
echo "  .harness/bin/harness-control"
