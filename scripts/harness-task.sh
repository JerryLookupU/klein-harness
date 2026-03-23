#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF' >&2
usage: harness-task <ROOT> <TASK_ID|REQUEST_ID> [detail|logs] [options...]

Canonical task detail / lineage / log surface for Klein-Harness.
EOF
}

if [[ $# -lt 2 ]]; then
  usage
  exit 1
fi

ROOT_INPUT="$1"
ITEM_ID="$2"
shift 2

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
LOCAL_TASK="$ROOT/.harness/bin/harness-task"
LOCAL_QUERY="$ROOT/.harness/bin/harness-query"
LOCAL_OPS="$ROOT/.harness/bin/harness-ops"

if [[ -x "$LOCAL_TASK" ]]; then
  exec "$LOCAL_TASK" "$ROOT" "$ITEM_ID" "$@"
fi

if [[ ! -x "$LOCAL_QUERY" || ! -x "$LOCAL_OPS" ]]; then
  echo "project not initialized: $ROOT" >&2
  echo "hint: run harness-submit \"$ROOT\" --goal \"<GOAL>\"" >&2
  exit 1
fi

MODE="detail"
OPS_FORMAT_ARGS=()
QUERY_FORMAT_ARGS=()
PASSTHRU=()
while [[ $# -gt 0 ]]; do
  case "$1" in
    detail|logs|log)
      MODE="$1"
      shift
      ;;
    --format)
      if [[ "${2:-json}" == "text" ]]; then
        QUERY_FORMAT_ARGS+=("--text")
        OPS_FORMAT_ARGS+=("--format" "text")
      else
        OPS_FORMAT_ARGS+=("--format" "$2")
      fi
      shift 2
      ;;
    *)
      PASSTHRU+=("$1")
      shift
      ;;
  esac
done

if [[ "$ITEM_ID" == R-* ]]; then
  if [[ "$MODE" == "logs" || "$MODE" == "log" ]]; then
    if [[ ${#QUERY_FORMAT_ARGS[@]} -gt 0 && ${#PASSTHRU[@]} -gt 0 ]]; then
      exec "$LOCAL_QUERY" logs "$ROOT" --request-id "$ITEM_ID" "${QUERY_FORMAT_ARGS[@]}" "${PASSTHRU[@]}"
    elif [[ ${#QUERY_FORMAT_ARGS[@]} -gt 0 ]]; then
      exec "$LOCAL_QUERY" logs "$ROOT" --request-id "$ITEM_ID" "${QUERY_FORMAT_ARGS[@]}"
    elif [[ ${#PASSTHRU[@]} -gt 0 ]]; then
      exec "$LOCAL_QUERY" logs "$ROOT" --request-id "$ITEM_ID" "${PASSTHRU[@]}"
    else
      exec "$LOCAL_QUERY" logs "$ROOT" --request-id "$ITEM_ID"
    fi
  fi
  if [[ ${#OPS_FORMAT_ARGS[@]} -gt 0 && ${#PASSTHRU[@]} -gt 0 ]]; then
    exec "$LOCAL_OPS" "$ROOT" "${OPS_FORMAT_ARGS[@]}" request "$ITEM_ID" "${PASSTHRU[@]}"
  elif [[ ${#OPS_FORMAT_ARGS[@]} -gt 0 ]]; then
    exec "$LOCAL_OPS" "$ROOT" "${OPS_FORMAT_ARGS[@]}" request "$ITEM_ID"
  elif [[ ${#PASSTHRU[@]} -gt 0 ]]; then
    exec "$LOCAL_OPS" "$ROOT" request "$ITEM_ID" "${PASSTHRU[@]}"
  else
    exec "$LOCAL_OPS" "$ROOT" request "$ITEM_ID"
  fi
fi

if [[ "$MODE" == "logs" || "$MODE" == "log" ]]; then
  if [[ ${#QUERY_FORMAT_ARGS[@]} -gt 0 && ${#PASSTHRU[@]} -gt 0 ]]; then
    exec "$LOCAL_QUERY" log "$ROOT" "$ITEM_ID" "${QUERY_FORMAT_ARGS[@]}" "${PASSTHRU[@]}"
  elif [[ ${#QUERY_FORMAT_ARGS[@]} -gt 0 ]]; then
    exec "$LOCAL_QUERY" log "$ROOT" "$ITEM_ID" "${QUERY_FORMAT_ARGS[@]}"
  elif [[ ${#PASSTHRU[@]} -gt 0 ]]; then
    exec "$LOCAL_QUERY" log "$ROOT" "$ITEM_ID" "${PASSTHRU[@]}"
  else
    exec "$LOCAL_QUERY" log "$ROOT" "$ITEM_ID"
  fi
fi

if [[ ${#QUERY_FORMAT_ARGS[@]} -gt 0 && ${#PASSTHRU[@]} -gt 0 ]]; then
  exec "$LOCAL_QUERY" task "$ROOT" "$ITEM_ID" "${QUERY_FORMAT_ARGS[@]}" "${PASSTHRU[@]}"
elif [[ ${#QUERY_FORMAT_ARGS[@]} -gt 0 ]]; then
  exec "$LOCAL_QUERY" task "$ROOT" "$ITEM_ID" "${QUERY_FORMAT_ARGS[@]}"
elif [[ ${#PASSTHRU[@]} -gt 0 ]]; then
  exec "$LOCAL_QUERY" task "$ROOT" "$ITEM_ID" "${PASSTHRU[@]}"
else
  exec "$LOCAL_QUERY" task "$ROOT" "$ITEM_ID"
fi
