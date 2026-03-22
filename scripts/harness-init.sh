#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF' >&2
usage: harness-init <ROOT>

Create the minimal .harness runtime skeleton without invoking a model.
EOF
}

if [[ $# -ne 1 ]]; then
  usage
  exit 1
fi

ROOT_INPUT="$1"
if [[ "$ROOT_INPUT" = /* ]]; then
  ROOT="$ROOT_INPUT"
else
  ROOT="$(pwd)/$ROOT_INPUT"
fi

CODEX_BASE="${CODEX_HOME:-$HOME/.codex}"
SKILL_DIR="$CODEX_BASE/skills/klein-harness"
INSTALL_FULL_SH="$SKILL_DIR/examples/harness-install-full.example.sh"

if [[ ! -d "$SKILL_DIR" ]]; then
  echo "klein-harness skill is not installed: $SKILL_DIR" >&2
  echo "run: ./install.sh --force" >&2
  exit 1
fi

if [[ ! -f "$INSTALL_FULL_SH" ]]; then
  echo "install helper missing: $INSTALL_FULL_SH" >&2
  exit 1
fi

mkdir -p "$ROOT"
ROOT="$(cd "$ROOT" && pwd)"

if [[ ! -x "$INSTALL_FULL_SH" ]]; then
  chmod +x "$INSTALL_FULL_SH"
fi

bash "$INSTALL_FULL_SH" "$ROOT"

echo "initialized: $ROOT"
echo "runtime root: $ROOT/.harness"
echo "next:"
echo "  harness-bootstrap \"$ROOT\" \"<GOAL>\""
