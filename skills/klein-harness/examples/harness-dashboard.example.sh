#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -lt 1 ]; then
  echo "usage: $0 <ROOT> [TASK_ID] [--watch SECONDS]" >&2
  exit 1
fi

ROOT="$1"
TASK_ID="${2:-}"
WATCH_FLAG="${3:-}"
INTERVAL="${4:-2}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
QUERY_SH=""

if [ -x "$SCRIPT_DIR/harness-query" ]; then
  QUERY_SH="$SCRIPT_DIR/harness-query"
elif [ -x "$SCRIPT_DIR/harness-query.example.sh" ]; then
  QUERY_SH="$SCRIPT_DIR/harness-query.example.sh"
else
  echo "harness-query wrapper not found" >&2
  exit 1
fi

if [ "$TASK_ID" = "--watch" ]; then
  TASK_ID=""
  WATCH_FLAG="--watch"
  INTERVAL="${3:-2}"
fi

render_once() {
  if [ -t 1 ]; then
    clear
  fi
  echo "== Harness Dashboard =="
  echo "root: $ROOT"
  if [ -n "$TASK_ID" ]; then
    echo "task: $TASK_ID"
  fi
  echo

  echo "-- Overview --"
  "$QUERY_SH" overview "$ROOT" --text
  echo

  echo "-- Current --"
  "$QUERY_SH" current "$ROOT" --text
  echo

  echo "-- Progress --"
  "$QUERY_SH" progress "$ROOT" --text
  echo

  echo "-- Feedback --"
  "$QUERY_SH" feedback "$ROOT" --text
  echo

  echo "-- Blueprint --"
  "$QUERY_SH" blueprint "$ROOT" --text
  echo

  if [ -n "$TASK_ID" ]; then
    echo "-- Task --"
    "$QUERY_SH" task "$ROOT" "$TASK_ID" --text
    echo
  fi
}

if [ "$WATCH_FLAG" = "--watch" ]; then
  while true; do
    render_once
    sleep "$INTERVAL"
  done
else
  render_once
fi
