#!/usr/bin/env bash
set -euo pipefail

POSITIONAL=()
CONTEXTS=()
RUN_DAEMON=1

while [[ $# -gt 0 ]]; do
  case "$1" in
    --context|--prd)
      CONTEXTS+=("$2")
      shift 2
      ;;
    --no-daemon|--manual|--no-bootstrap|--no-session-init|--replace-bootstrap)
      if [[ "$1" == "--no-daemon" || "$1" == "--manual" || "$1" == "--no-bootstrap" ]]; then
        RUN_DAEMON=0
      fi
      shift
      ;;
    --daemon)
      RUN_DAEMON=1
      shift
      ;;
    --model|--concurrency|--daemon-interval)
      shift 2
      ;;
    -h|--help)
      echo "usage: harness-kick [options] <GOAL> [STACK_HINT] [ROOT]" >&2
      exit 0
      ;;
    *)
      POSITIONAL+=("$1")
      shift
      ;;
  esac
done

if [[ ${#POSITIONAL[@]} -lt 1 ]]; then
  echo "usage: harness-kick [options] <GOAL> [STACK_HINT] [ROOT]" >&2
  exit 1
fi

GOAL="${POSITIONAL[0]}"
STACK_HINT="${POSITIONAL[1]:-}"
ROOT="${POSITIONAL[2]:-.}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

RUNNER=(go run "${REPO_ROOT}/cmd/harness")
if [[ -n "${HARNESS_BIN:-}" ]]; then
  RUNNER=("${HARNESS_BIN}")
elif command -v harness >/dev/null 2>&1; then
  RUNNER=(harness)
fi

SUBMIT_ARGS=("$ROOT" --goal "$GOAL")
if [[ -n "$STACK_HINT" ]]; then
  SUBMIT_ARGS+=(--context "$STACK_HINT")
fi
for context in "${CONTEXTS[@]}"; do
  SUBMIT_ARGS+=(--context "$context")
done

"${RUNNER[@]}" submit "${SUBMIT_ARGS[@]}"

if [[ "$RUN_DAEMON" -eq 1 ]]; then
  exec "${RUNNER[@]}" daemon run-once "$ROOT"
fi
