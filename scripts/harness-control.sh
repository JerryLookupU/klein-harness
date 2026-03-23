#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF' >&2
usage: harness-control <ROOT> <daemon|task|request|project> [args...]

Canonical control surface for Klein-Harness.
Primary task control actions:
  checkpoint
  archive
  stop
  restart-from-stage <queued|worktree_prepared|merge_queued>
  restart [--from-stage <stage>]    # legacy compatibility alias to restart-from-stage

Compatibility actions (kept for power users):
  retry
  recover
  force-recover
  attach
  request cancel
  project archive
  project tidy-worktrees [--dry-run]

Examples:
  harness-control /repo daemon status
  harness-control /repo task T-003 checkpoint --reason "safe pause"
  harness-control /repo task T-003 restart-from-stage queued --reason "retry from clean stage"
  harness-control /repo task T-003 stop --reason "operator stop"
  harness-control /repo project archive --reason "loop retired"
  harness-control /repo project tidy-worktrees --dry-run
EOF
}

if [[ $# -lt 2 ]]; then
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

ROOT="$(cd "$ROOT" 2>/dev/null && pwd || true)"
if [[ -z "${ROOT:-}" ]]; then
  echo "project root not found: $ROOT_INPUT" >&2
  exit 1
fi

LOCAL_CONTROL="$ROOT/.harness/bin/harness-control"
LOCAL_OPS="$ROOT/.harness/bin/harness-ops"
LOCAL_RUNNER="$ROOT/.harness/bin/harness-runner"
LOCAL_REQUEST="$ROOT/.harness/bin/harness-submit"

if [[ -x "$LOCAL_CONTROL" ]]; then
  exec "$LOCAL_CONTROL" "$ROOT" "$@"
fi

if [[ ! -x "$LOCAL_OPS" || ! -x "$LOCAL_RUNNER" || ! -x "$LOCAL_REQUEST" ]]; then
  echo "project not initialized: $ROOT" >&2
  echo "hint: run harness-submit \"$ROOT\" --goal \"<GOAL>\"" >&2
  exit 1
fi

DOMAIN="$1"
shift
FORMAT_ARGS=()
PASSTHRU=()
while [[ $# -gt 0 ]]; do
  case "$1" in
    --format)
      FORMAT_ARGS+=("$1" "$2")
      shift 2
      ;;
    *)
      PASSTHRU+=("$1")
      shift
      ;;
  esac
done

case "$DOMAIN" in
  daemon)
    if [[ ${#FORMAT_ARGS[@]} -gt 0 && ${#PASSTHRU[@]} -gt 0 ]]; then
      exec "$LOCAL_OPS" "$ROOT" "${FORMAT_ARGS[@]}" daemon "${PASSTHRU[@]}"
    elif [[ ${#FORMAT_ARGS[@]} -gt 0 ]]; then
      exec "$LOCAL_OPS" "$ROOT" "${FORMAT_ARGS[@]}" daemon
    elif [[ ${#PASSTHRU[@]} -gt 0 ]]; then
      exec "$LOCAL_OPS" "$ROOT" daemon "${PASSTHRU[@]}"
    else
      exec "$LOCAL_OPS" "$ROOT" daemon
    fi
    ;;
  task)
    if [[ ${#PASSTHRU[@]} -lt 2 ]]; then
      echo "usage: harness-control <ROOT> task <TASK_ID> <checkpoint|archive|stop|restart-from-stage|restart|attach> [args...] [ --from-stage <stage> ]" >&2
      echo "Legacy compatibility actions: retry | recover | force-recover" >&2
      exit 1
    fi
    TASK_ID="${PASSTHRU[0]}"
    ACTION="${PASSTHRU[1]}"
    REST=("${PASSTHRU[@]:2}")
    case "$ACTION" in
      retry|recover|force-recover)
        if [[ ${#REST[@]} -gt 0 ]]; then
          exec "$LOCAL_RUNNER" recover "$TASK_ID" "$ROOT" "${REST[@]}"
        fi
        exec "$LOCAL_RUNNER" recover "$TASK_ID" "$ROOT"
        ;;
      restart)
        if [[ ${#REST[@]} -gt 0 && "${REST[0]}" == "--from-stage" ]]; then
          CONTROL_SCRIPT="$ROOT/.harness/scripts/control.py"
          exec python3 "$CONTROL_SCRIPT" --root "$ROOT" "${FORMAT_ARGS[@]}" task "$TASK_ID" restart "${REST[@]}"
        fi
        if [[ ${#REST[@]} -gt 0 ]]; then
          exec "$LOCAL_RUNNER" run "$TASK_ID" "$ROOT" "${REST[@]}"
        fi
        exec "$LOCAL_RUNNER" run "$TASK_ID" "$ROOT"
        ;;
      restart-from-stage)
        CONTROL_SCRIPT="$ROOT/.harness/scripts/control.py"
        if [[ ! -f "$CONTROL_SCRIPT" ]]; then
          echo "control script not found: $CONTROL_SCRIPT" >&2
          exit 1
        fi
        if [[ ${#REST[@]} -lt 1 ]]; then
          echo "usage: harness-control <ROOT> task <TASK_ID> restart-from-stage <queued|worktree_prepared|merge_queued> [--reason ...]" >&2
          exit 1
        fi
        STAGE="${REST[0]}"
        EXTRA=("${REST[@]:1}")
        if [[ ${#FORMAT_ARGS[@]} -gt 0 && ${#EXTRA[@]} -gt 0 ]]; then
          exec python3 "$CONTROL_SCRIPT" --root "$ROOT" "${FORMAT_ARGS[@]}" task "$TASK_ID" restart --from-stage "$STAGE" "${EXTRA[@]}"
        elif [[ ${#FORMAT_ARGS[@]} -gt 0 ]]; then
          exec python3 "$CONTROL_SCRIPT" --root "$ROOT" "${FORMAT_ARGS[@]}" task "$TASK_ID" restart --from-stage "$STAGE"
        elif [[ ${#EXTRA[@]} -gt 0 ]]; then
          exec python3 "$CONTROL_SCRIPT" --root "$ROOT" task "$TASK_ID" restart --from-stage "$STAGE" "${EXTRA[@]}"
        fi
        exec python3 "$CONTROL_SCRIPT" --root "$ROOT" task "$TASK_ID" restart --from-stage "$STAGE"
        ;;
      attach)
        exec "$LOCAL_RUNNER" attach "$TASK_ID" "$ROOT"
        ;;
      checkpoint|archive|stop)
        CONTROL_SCRIPT="$ROOT/.harness/scripts/control.py"
        if [[ ! -f "$CONTROL_SCRIPT" ]]; then
          echo "control script not found: $CONTROL_SCRIPT" >&2
          exit 1
        fi
        if [[ ${#FORMAT_ARGS[@]} -gt 0 && ${#REST[@]} -gt 0 ]]; then
          exec python3 "$CONTROL_SCRIPT" --root "$ROOT" "${FORMAT_ARGS[@]}" task "$TASK_ID" "$ACTION" "${REST[@]}"
        elif [[ ${#FORMAT_ARGS[@]} -gt 0 ]]; then
          exec python3 "$CONTROL_SCRIPT" --root "$ROOT" "${FORMAT_ARGS[@]}" task "$TASK_ID" "$ACTION"
        elif [[ ${#REST[@]} -gt 0 ]]; then
          exec python3 "$CONTROL_SCRIPT" --root "$ROOT" task "$TASK_ID" "$ACTION" "${REST[@]}"
        fi
        exec python3 "$CONTROL_SCRIPT" --root "$ROOT" task "$TASK_ID" "$ACTION"
        ;;
      *)
        echo "unknown task action: $ACTION" >&2
        exit 1
        ;;
    esac
    ;;
  request)
    CONTROL_SCRIPT="$ROOT/.harness/scripts/control.py"
    if [[ ! -f "$CONTROL_SCRIPT" ]]; then
      echo "control script not found: $CONTROL_SCRIPT" >&2
      exit 1
    fi
    if [[ ${#FORMAT_ARGS[@]} -gt 0 && ${#PASSTHRU[@]} -gt 0 ]]; then
      exec python3 "$CONTROL_SCRIPT" --root "$ROOT" "${FORMAT_ARGS[@]}" request "${PASSTHRU[@]}"
    elif [[ ${#FORMAT_ARGS[@]} -gt 0 ]]; then
      exec python3 "$CONTROL_SCRIPT" --root "$ROOT" "${FORMAT_ARGS[@]}" request
    elif [[ ${#PASSTHRU[@]} -gt 0 ]]; then
      exec python3 "$CONTROL_SCRIPT" --root "$ROOT" request "${PASSTHRU[@]}"
    fi
    exec python3 "$CONTROL_SCRIPT" --root "$ROOT" request
    ;;
  project)
    CONTROL_SCRIPT="$ROOT/.harness/scripts/control.py"
    if [[ ! -f "$CONTROL_SCRIPT" ]]; then
      echo "control script not found: $CONTROL_SCRIPT" >&2
      exit 1
    fi
    if [[ ${#FORMAT_ARGS[@]} -gt 0 && ${#PASSTHRU[@]} -gt 0 ]]; then
      exec python3 "$CONTROL_SCRIPT" --root "$ROOT" "${FORMAT_ARGS[@]}" project "${PASSTHRU[@]}"
    elif [[ ${#FORMAT_ARGS[@]} -gt 0 ]]; then
      exec python3 "$CONTROL_SCRIPT" --root "$ROOT" "${FORMAT_ARGS[@]}" project
    elif [[ ${#PASSTHRU[@]} -gt 0 ]]; then
      exec python3 "$CONTROL_SCRIPT" --root "$ROOT" project "${PASSTHRU[@]}"
    fi
    exec python3 "$CONTROL_SCRIPT" --root "$ROOT" project
    ;;
  *)
    echo "unknown control domain: $DOMAIN" >&2
    exit 1
    ;;
esac
