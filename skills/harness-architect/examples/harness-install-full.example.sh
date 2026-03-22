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
STATE_DIR="$HARNESS_DIR/state"
TEMPLATES_DIR="$HARNESS_DIR/templates"
REQUESTS=(
  "$HARNESS_DIR/audit-requests.json"
  "$HARNESS_DIR/replan-requests.json"
  "$HARNESS_DIR/stop-requests.json"
)
MANIFEST="$HARNESS_DIR/tooling-manifest.json"
EXAMPLES_DIR="$(cd "$(dirname "$0")" && pwd)"
REQUEST_QUEUE_PATH="$HARNESS_DIR/requests/queue.jsonl"
REQUEST_ARCHIVE_DIR="$HARNESS_DIR/requests/archive"
REQUEST_INDEX_PATH="$STATE_DIR/request-index.json"
REQUEST_TASK_MAP_PATH="$STATE_DIR/request-task-map.json"
PROJECT_META_PATH="$HARNESS_DIR/project-meta.json"
FEEDBACK_LOG_PATH="$HARNESS_DIR/feedback-log.jsonl"
LINEAGE_PATH="$HARNESS_DIR/lineage.jsonl"
FEEDBACK_SUMMARY_PATH="$STATE_DIR/feedback-summary.json"
RUNNER_STATE_PATH="$STATE_DIR/runner-state.json"
RUNNER_HEARTBEATS_PATH="$STATE_DIR/runner-heartbeats.json"
REQUEST_SUMMARY_PATH="$STATE_DIR/request-summary.json"
LINEAGE_INDEX_PATH="$STATE_DIR/lineage-index.json"

mkdir -p "$BIN_DIR" "$SCRIPTS_DIR" "$STATE_DIR" "$TEMPLATES_DIR" "$HARNESS_DIR/drift-log" "$HARNESS_DIR/verification-rules" "$STATE_DIR/runner-logs" "$REQUEST_ARCHIVE_DIR"

install_file() {
  local src="$1"
  local dst="$2"
  cp "$EXAMPLES_DIR/$src" "$dst"
}

install_file "harness-query.example.sh" "$BIN_DIR/harness-query"
install_file "harness-dashboard.example.sh" "$BIN_DIR/harness-dashboard"
install_file "harness-status.example.sh" "$BIN_DIR/harness-status"
install_file "harness-watch.example.sh" "$BIN_DIR/harness-watch"
install_file "harness-render-prompt.example.sh" "$BIN_DIR/harness-render-prompt"
install_file "harness-route-session.example.sh" "$BIN_DIR/harness-route-session"
install_file "harness-prepare-worktree.example.sh" "$BIN_DIR/harness-prepare-worktree"
install_file "harness-diff-summary.example.sh" "$BIN_DIR/harness-diff-summary"
install_file "harness-verify-task.example.sh" "$BIN_DIR/harness-verify-task"
install_file "harness-runner.example.sh" "$BIN_DIR/harness-runner"
install_file "harness-submit.example.sh" "$BIN_DIR/harness-submit"
install_file "harness-report.example.sh" "$BIN_DIR/harness-report"

install_file "query-harness.example.py" "$SCRIPTS_DIR/query.py"
install_file "refresh-state.example.py" "$SCRIPTS_DIR/refresh-state.py"
install_file "status.example.py" "$SCRIPTS_DIR/status.py"
install_file "render-prompt.example.py" "$SCRIPTS_DIR/render-prompt.py"
install_file "route-session.example.py" "$SCRIPTS_DIR/route-session.py"
install_file "prepare-worktree.example.py" "$SCRIPTS_DIR/prepare-worktree.py"
install_file "diff-summary.example.py" "$SCRIPTS_DIR/diff-summary.py"
install_file "verify-task.example.py" "$SCRIPTS_DIR/verify-task.py"
install_file "runner.example.py" "$SCRIPTS_DIR/runner.py"
install_file "request.example.py" "$SCRIPTS_DIR/request.py"
install_file "runtime-common.example.py" "$SCRIPTS_DIR/runtime_common.py"

install_file "session-init.example.sh" "$HARNESS_DIR/session-init.sh"
install_file "AGENTS.example.md" "$TEMPLATES_DIR/AGENTS.template.md"

chmod +x \
  "$BIN_DIR/harness-query" \
  "$BIN_DIR/harness-dashboard" \
  "$BIN_DIR/harness-status" \
  "$BIN_DIR/harness-watch" \
  "$BIN_DIR/harness-render-prompt" \
  "$BIN_DIR/harness-route-session" \
  "$BIN_DIR/harness-prepare-worktree" \
  "$BIN_DIR/harness-diff-summary" \
  "$BIN_DIR/harness-verify-task" \
  "$BIN_DIR/harness-runner" \
  "$BIN_DIR/harness-submit" \
  "$BIN_DIR/harness-report" \
  "$HARNESS_DIR/session-init.sh"

for request_path in "${REQUESTS[@]}"; do
  if [ ! -f "$request_path" ]; then
    printf '{\n  "schemaVersion": "1.0",\n  "generator": "harness-architect",\n  "generatedAt": null,\n  "requests": []\n}\n' > "$request_path"
  fi
done

if [ ! -f "$REQUEST_QUEUE_PATH" ]; then
  : > "$REQUEST_QUEUE_PATH"
fi

if [ ! -f "$FEEDBACK_LOG_PATH" ]; then
  : > "$FEEDBACK_LOG_PATH"
fi

if [ ! -f "$LINEAGE_PATH" ]; then
  : > "$LINEAGE_PATH"
fi

python3 - <<'PY' "$ROOT" "$SCRIPTS_DIR/runtime_common.py"
import importlib.util
import sys
from pathlib import Path

root = Path(sys.argv[1]).resolve()
module_path = Path(sys.argv[2]).resolve()
spec = importlib.util.spec_from_file_location("runtime_common", module_path)
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)
module.ensure_runtime_scaffold(root, generator="harness-architect")
PY

cat > "$MANIFEST" <<'JSON'
{
  "schemaVersion": "1.0",
  "generator": "harness-architect",
  "generatedAt": "INSTALL_TIME",
  "installed": [
    {
      "name": "harness-query",
      "target": ".harness/bin/harness-query",
      "source": "examples/harness-query.example.sh"
    },
    {
      "name": "harness-dashboard",
      "target": ".harness/bin/harness-dashboard",
      "source": "examples/harness-dashboard.example.sh"
    },
    {
      "name": "harness-status",
      "target": ".harness/bin/harness-status",
      "source": "examples/harness-status.example.sh"
    },
    {
      "name": "harness-watch",
      "target": ".harness/bin/harness-watch",
      "source": "examples/harness-watch.example.sh"
    },
    {
      "name": "harness-render-prompt",
      "target": ".harness/bin/harness-render-prompt",
      "source": "examples/harness-render-prompt.example.sh"
    },
    {
      "name": "harness-route-session",
      "target": ".harness/bin/harness-route-session",
      "source": "examples/harness-route-session.example.sh"
    },
    {
      "name": "harness-prepare-worktree",
      "target": ".harness/bin/harness-prepare-worktree",
      "source": "examples/harness-prepare-worktree.example.sh"
    },
    {
      "name": "harness-diff-summary",
      "target": ".harness/bin/harness-diff-summary",
      "source": "examples/harness-diff-summary.example.sh"
    },
    {
      "name": "harness-verify-task",
      "target": ".harness/bin/harness-verify-task",
      "source": "examples/harness-verify-task.example.sh"
    },
    {
      "name": "harness-submit",
      "target": ".harness/bin/harness-submit",
      "source": "examples/harness-submit.example.sh"
    },
    {
      "name": "harness-report",
      "target": ".harness/bin/harness-report",
      "source": "examples/harness-report.example.sh"
    },
    {
      "name": "query.py",
      "target": ".harness/scripts/query.py",
      "source": "examples/query-harness.example.py"
    },
    {
      "name": "refresh-state.py",
      "target": ".harness/scripts/refresh-state.py",
      "source": "examples/refresh-state.example.py"
    },
    {
      "name": "status.py",
      "target": ".harness/scripts/status.py",
      "source": "examples/status.example.py"
    },
    {
      "name": "render-prompt.py",
      "target": ".harness/scripts/render-prompt.py",
      "source": "examples/render-prompt.example.py"
    },
    {
      "name": "route-session.py",
      "target": ".harness/scripts/route-session.py",
      "source": "examples/route-session.example.py"
    },
    {
      "name": "prepare-worktree.py",
      "target": ".harness/scripts/prepare-worktree.py",
      "source": "examples/prepare-worktree.example.py"
    },
    {
      "name": "diff-summary.py",
      "target": ".harness/scripts/diff-summary.py",
      "source": "examples/diff-summary.example.py"
    },
    {
      "name": "verify-task.py",
      "target": ".harness/scripts/verify-task.py",
      "source": "examples/verify-task.example.py"
    },
    {
      "name": "request.py",
      "target": ".harness/scripts/request.py",
      "source": "examples/request.example.py"
    },
    {
      "name": "runtime_common.py",
      "target": ".harness/scripts/runtime_common.py",
      "source": "examples/runtime-common.example.py"
    },
    {
      "name": "session-init.sh",
      "target": ".harness/session-init.sh",
      "source": "examples/session-init.example.sh"
    },
    {
      "name": "AGENTS.template.md",
      "target": ".harness/templates/AGENTS.template.md",
      "source": "examples/AGENTS.example.md"
    },
    {
      "name": "harness-runner",
      "target": ".harness/bin/harness-runner",
      "source": "examples/harness-runner.example.sh"
    },
    {
      "name": "runner.py",
      "target": ".harness/scripts/runner.py",
      "source": "examples/runner.example.py"
    }
  ]
}
JSON

python3 - <<'PY' "$MANIFEST" "${REQUESTS[@]}" "$REQUEST_SUMMARY_PATH" "$LINEAGE_INDEX_PATH"
import json
import sys
from datetime import datetime, timezone
from pathlib import Path

manifest_path = sys.argv[1]
request_paths = sys.argv[2:5]
state_paths = sys.argv[5:]
timestamp = datetime.now(timezone.utc).astimezone().isoformat(timespec="seconds")

manifest = json.load(open(manifest_path))
manifest["generatedAt"] = timestamp
json.dump(manifest, open(manifest_path, "w"), ensure_ascii=False, indent=2)
open(manifest_path, "a").write("\n")

for path in request_paths:
    data = json.load(open(path))
    data["generatedAt"] = timestamp
    json.dump(data, open(path, "w"), ensure_ascii=False, indent=2)
    open(path, "a").write("\n")

for path in state_paths:
    if not Path(path).exists():
        continue
    data = json.load(open(path))
    data["generatedAt"] = timestamp
    json.dump(data, open(path, "w"), ensure_ascii=False, indent=2)
    open(path, "a").write("\n")
PY

echo "installed full harness operator toolset into $HARNESS_DIR"
