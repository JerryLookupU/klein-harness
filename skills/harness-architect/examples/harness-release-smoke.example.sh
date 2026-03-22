#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
INSTALL_SH="$SCRIPT_DIR/harness-install-full.example.sh"

TMP_ROOT="$(mktemp -d)"
PROJECT_ROOT="$TMP_ROOT/release-smoke-project"
mkdir -p "$PROJECT_ROOT/.harness"

cleanup() {
  rm -rf "$TMP_ROOT"
}
trap cleanup EXIT

"$INSTALL_SH" "$PROJECT_ROOT" >/dev/null

cp "$SCRIPT_DIR/progress.example.md" "$PROJECT_ROOT/.harness/progress.md"
cp "$SCRIPT_DIR/task-pool.example.json" "$PROJECT_ROOT/.harness/task-pool.json"
cp "$SCRIPT_DIR/work-items.example.json" "$PROJECT_ROOT/.harness/work-items.json"
cp "$SCRIPT_DIR/features.example.json" "$PROJECT_ROOT/.harness/features.json"
cp "$SCRIPT_DIR/spec.example.json" "$PROJECT_ROOT/.harness/spec.json"
cp "$SCRIPT_DIR/session-registry.example.json" "$PROJECT_ROOT/.harness/session-registry.json"
cp "$SCRIPT_DIR/current-state.example.json" "$PROJECT_ROOT/.harness/state/current-state.json"
cp "$SCRIPT_DIR/runtime-state.example.json" "$PROJECT_ROOT/.harness/state/runtime-state.json"
cp "$SCRIPT_DIR/feedback-log.example.jsonl" "$PROJECT_ROOT/.harness/feedback-log.jsonl"

python3 "$PROJECT_ROOT/.harness/scripts/refresh-state.py" "$PROJECT_ROOT" >/dev/null
python3 "$PROJECT_ROOT/.harness/scripts/query.py" --root "$PROJECT_ROOT" --view feedback >/dev/null
python3 "$PROJECT_ROOT/.harness/scripts/status.py" --root "$PROJECT_ROOT" >/dev/null
python3 "$PROJECT_ROOT/.harness/scripts/route-session.py" --root "$PROJECT_ROOT" --task-id T-002 >/dev/null
python3 "$PROJECT_ROOT/.harness/scripts/render-prompt.py" --root "$PROJECT_ROOT" --task-id T-003 --role worker --stage start >/dev/null
python3 "$PROJECT_ROOT/.harness/scripts/runner.py" run T-003 "$PROJECT_ROOT" --dispatch-mode print >/dev/null

echo "release smoke passed"
