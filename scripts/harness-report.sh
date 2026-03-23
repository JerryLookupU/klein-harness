#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF' >&2
usage: harness-report <ROOT> [options...]
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

LOCAL_REPORT="$ROOT/.harness/bin/harness-report"
FORMAT="text"

ARGS=("$@")
index=0
while [[ $index -lt ${#ARGS[@]} ]]; do
  if [[ "${ARGS[$index]}" == "--format" && $((index + 1)) -lt ${#ARGS[@]} ]]; then
    FORMAT="${ARGS[$((index + 1))]}"
    break
  fi
  index=$((index + 1))
done

if [[ ! -x "$LOCAL_REPORT" ]]; then
  if [[ "$FORMAT" == "json" ]]; then
    cat <<EOF
{
  "projectRoot": "$ROOT",
  "status": "project_not_initialized",
  "hint": "run harness-submit \\\"$ROOT\\\" --goal \\\"<GOAL>\\\""
}
EOF
  else
    echo "project: $ROOT"
    echo "status: project_not_initialized"
    echo "hint: run harness-submit \"$ROOT\" --goal \"<GOAL>\""
  fi
  exit 0
fi

exec "$LOCAL_REPORT" "$ROOT" "$@"
