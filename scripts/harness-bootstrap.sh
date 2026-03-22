#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF' >&2
usage: harness-bootstrap <ROOT> <GOAL> [STACK_HINT] [kick options...]

Examples:
  harness-bootstrap /repo "分析这个代码库"
  harness-bootstrap /repo "根据 PRD 生成代码" "Next.js + Prisma" --context docs/prd.md
  harness-bootstrap /repo "根据 PRD 生成代码" "Next.js + Prisma" --context docs/prd.md --no-daemon
EOF
}

if [[ $# -lt 2 ]]; then
  usage
  exit 1
fi

ROOT="$1"
GOAL="$2"
shift 2

STACK_HINT=""
if [[ $# -gt 0 && "$1" != -* ]]; then
  STACK_HINT="$1"
  shift
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
KICK="$SCRIPT_DIR/harness-kick"
if [[ ! -x "$KICK" && -x "$SCRIPT_DIR/harness-kick.sh" ]]; then
  KICK="$SCRIPT_DIR/harness-kick.sh"
fi

if [[ ! -x "$KICK" ]]; then
  echo "bootstrap helper not found: $KICK" >&2
  exit 1
fi

exec "$KICK" "$@" "$GOAL" "$STACK_HINT" "$ROOT"
