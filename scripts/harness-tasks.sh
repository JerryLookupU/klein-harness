#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF' >&2
usage: harness-tasks <ROOT> [summary|queue|tasks|requests|workers|daemon|blockers|logs|worktrees|merge-queue|integration|conflicts] [options...]

Canonical task list / queue / summary surface for Klein-Harness.

Examples:
  harness-tasks /repo
  harness-tasks /repo queue
  harness-tasks /repo workers --format json
EOF
}

if [[ $# -lt 1 ]]; then
  usage
  exit 1
fi

ROOT_INPUT="$1"
shift

if [[ "$ROOT_INPUT" = /* ]]; then
  ROOT="$ROOT_INPUT"
else
  ROOT="$(pwd)/$ROOT_INPUT"
fi

ROOT="$(cd "$ROOT" 2>/dev/null && pwd || true)"
if [[ -z "${ROOT:-}" ]]; then
  echo "project root not found: $ROOT_INPUT" >&2
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOCAL_TASKS="$ROOT/.harness/bin/harness-tasks"
LOCAL_OPS="$ROOT/.harness/bin/harness-ops"

if [[ -x "$LOCAL_TASKS" ]]; then
  exec "$LOCAL_TASKS" "$ROOT" "$@"
fi

if [[ ! -x "$LOCAL_OPS" ]]; then
  echo "project not initialized: $ROOT" >&2
  echo "hint: run harness-submit \"$ROOT\" --goal \"<GOAL>\"" >&2
  exit 1
fi

FORMAT_ARGS=()
PASSTHRU=()
VIEW=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --format)
      FORMAT_ARGS+=("$1" "$2")
      shift 2
      ;;
    summary|top)
      VIEW="top"
      shift
      ;;
    list|tasks|queue|requests|workers|daemon|blockers|logs|worktrees|merge-queue|integration|conflicts)
      VIEW="$1"
      shift
      ;;
    *)
      PASSTHRU+=("$1")
      shift
      ;;
  esac
done

if [[ -z "$VIEW" ]]; then
  VIEW="tasks"
fi

if [[ ${#FORMAT_ARGS[@]} -gt 0 && ${#PASSTHRU[@]} -gt 0 ]]; then
  exec "$LOCAL_OPS" "$ROOT" "${FORMAT_ARGS[@]}" "$VIEW" "${PASSTHRU[@]}"
elif [[ ${#FORMAT_ARGS[@]} -gt 0 ]]; then
  exec "$LOCAL_OPS" "$ROOT" "${FORMAT_ARGS[@]}" "$VIEW"
elif [[ ${#PASSTHRU[@]} -gt 0 ]]; then
  exec "$LOCAL_OPS" "$ROOT" "$VIEW" "${PASSTHRU[@]}"
else
  exec "$LOCAL_OPS" "$ROOT" "$VIEW"
fi
