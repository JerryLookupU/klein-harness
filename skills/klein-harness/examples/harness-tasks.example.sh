#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <ROOT> [summary|queue|tasks|requests|workers|daemon|blockers|logs|worktrees|merge-queue|integration|conflicts] [options...]" >&2
  exit 1
fi

ROOT="$1"
shift
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
OPS_SH="$SCRIPT_DIR/harness-ops"
if [[ ! -x "$OPS_SH" && -x "$SCRIPT_DIR/harness-ops.example.sh" ]]; then
  OPS_SH="$SCRIPT_DIR/harness-ops.example.sh"
fi

if [[ ! -x "$OPS_SH" ]]; then
  echo "harness-ops wrapper not found" >&2
  exit 1
fi

VIEW="tasks"
FORMAT_ARGS=()
PASSTHRU=()
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

if [[ ${#FORMAT_ARGS[@]} -gt 0 && ${#PASSTHRU[@]} -gt 0 ]]; then
  exec "$OPS_SH" "$ROOT" "${FORMAT_ARGS[@]}" "$VIEW" "${PASSTHRU[@]}"
elif [[ ${#FORMAT_ARGS[@]} -gt 0 ]]; then
  exec "$OPS_SH" "$ROOT" "${FORMAT_ARGS[@]}" "$VIEW"
elif [[ ${#PASSTHRU[@]} -gt 0 ]]; then
  exec "$OPS_SH" "$ROOT" "$VIEW" "${PASSTHRU[@]}"
else
  exec "$OPS_SH" "$ROOT" "$VIEW"
fi
