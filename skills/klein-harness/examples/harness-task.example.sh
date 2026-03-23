#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 2 ]]; then
  echo "usage: $0 <ROOT> <TASK_ID|REQUEST_ID> [detail|logs] [options...]" >&2
  exit 1
fi

ROOT="$1"
ITEM_ID="$2"
shift 2
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
QUERY_SH="$SCRIPT_DIR/harness-query"
OPS_SH="$SCRIPT_DIR/harness-ops"
if [[ ! -x "$QUERY_SH" && -x "$SCRIPT_DIR/harness-query.example.sh" ]]; then
  QUERY_SH="$SCRIPT_DIR/harness-query.example.sh"
fi
if [[ ! -x "$OPS_SH" && -x "$SCRIPT_DIR/harness-ops.example.sh" ]]; then
  OPS_SH="$SCRIPT_DIR/harness-ops.example.sh"
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
      exec "$QUERY_SH" logs "$ROOT" --request-id "$ITEM_ID" "${QUERY_FORMAT_ARGS[@]}" "${PASSTHRU[@]}"
    elif [[ ${#QUERY_FORMAT_ARGS[@]} -gt 0 ]]; then
      exec "$QUERY_SH" logs "$ROOT" --request-id "$ITEM_ID" "${QUERY_FORMAT_ARGS[@]}"
    elif [[ ${#PASSTHRU[@]} -gt 0 ]]; then
      exec "$QUERY_SH" logs "$ROOT" --request-id "$ITEM_ID" "${PASSTHRU[@]}"
    else
      exec "$QUERY_SH" logs "$ROOT" --request-id "$ITEM_ID"
    fi
  fi
  if [[ ${#OPS_FORMAT_ARGS[@]} -gt 0 && ${#PASSTHRU[@]} -gt 0 ]]; then
    exec "$OPS_SH" "$ROOT" "${OPS_FORMAT_ARGS[@]}" request "$ITEM_ID" "${PASSTHRU[@]}"
  elif [[ ${#OPS_FORMAT_ARGS[@]} -gt 0 ]]; then
    exec "$OPS_SH" "$ROOT" "${OPS_FORMAT_ARGS[@]}" request "$ITEM_ID"
  elif [[ ${#PASSTHRU[@]} -gt 0 ]]; then
    exec "$OPS_SH" "$ROOT" request "$ITEM_ID" "${PASSTHRU[@]}"
  else
    exec "$OPS_SH" "$ROOT" request "$ITEM_ID"
  fi
fi

if [[ "$MODE" == "logs" || "$MODE" == "log" ]]; then
  if [[ ${#QUERY_FORMAT_ARGS[@]} -gt 0 && ${#PASSTHRU[@]} -gt 0 ]]; then
    exec "$QUERY_SH" log "$ROOT" "$ITEM_ID" "${QUERY_FORMAT_ARGS[@]}" "${PASSTHRU[@]}"
  elif [[ ${#QUERY_FORMAT_ARGS[@]} -gt 0 ]]; then
    exec "$QUERY_SH" log "$ROOT" "$ITEM_ID" "${QUERY_FORMAT_ARGS[@]}"
  elif [[ ${#PASSTHRU[@]} -gt 0 ]]; then
    exec "$QUERY_SH" log "$ROOT" "$ITEM_ID" "${PASSTHRU[@]}"
  else
    exec "$QUERY_SH" log "$ROOT" "$ITEM_ID"
  fi
fi

if [[ ${#QUERY_FORMAT_ARGS[@]} -gt 0 && ${#PASSTHRU[@]} -gt 0 ]]; then
  exec "$QUERY_SH" task "$ROOT" "$ITEM_ID" "${QUERY_FORMAT_ARGS[@]}" "${PASSTHRU[@]}"
elif [[ ${#QUERY_FORMAT_ARGS[@]} -gt 0 ]]; then
  exec "$QUERY_SH" task "$ROOT" "$ITEM_ID" "${QUERY_FORMAT_ARGS[@]}"
elif [[ ${#PASSTHRU[@]} -gt 0 ]]; then
  exec "$QUERY_SH" task "$ROOT" "$ITEM_ID" "${PASSTHRU[@]}"
else
  exec "$QUERY_SH" task "$ROOT" "$ITEM_ID"
fi
