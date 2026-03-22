#!/usr/bin/env bash
set -euo pipefail

ROOT="${1:-$(pwd)}"
INTERVAL="${2:-2}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
STATUS_SH=""

if [ -x "$SCRIPT_DIR/harness-status" ]; then
  STATUS_SH="$SCRIPT_DIR/harness-status"
elif [ -x "$SCRIPT_DIR/harness-status.example.sh" ]; then
  STATUS_SH="$SCRIPT_DIR/harness-status.example.sh"
else
  echo "harness-status wrapper not found" >&2
  exit 1
fi

while true; do
  if [ -t 1 ]; then
    clear
  fi
  "$STATUS_SH" "$ROOT"
  sleep "$INTERVAL"
done
