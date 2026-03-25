#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 2 ]]; then
  echo "usage: harness-bootstrap <ROOT> <GOAL> [STACK_HINT] [--context <PATH> ...]" >&2
  exit 1
fi

ROOT="$1"
GOAL="$2"
shift 2

ARGS=("$ROOT" --goal "$GOAL")
if [[ $# -gt 0 && "${1:-}" != -* ]]; then
  ARGS+=(--context "$1")
  shift
fi
ARGS+=("$@")

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

if [[ -n "${HARNESS_BIN:-}" ]]; then
  exec "${HARNESS_BIN}" submit "${ARGS[@]}"
fi

if command -v harness >/dev/null 2>&1; then
  exec harness submit "${ARGS[@]}"
fi

exec go run "${REPO_ROOT}/cmd/harness" submit "${ARGS[@]}"
