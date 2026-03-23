#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 2 ]]; then
  cat <<'EOF' >&2
usage: harness-control <ROOT> <daemon|task|request|project> [args...]

Examples:
  harness-control /repo daemon status
  harness-control /repo task T-003 checkpoint --reason "safe pause"
  harness-control /repo task T-003 restart-from-stage queued --reason "retry from clean stage"
  harness-control /repo task T-003 stop --reason "operator stop"
  harness-control /repo project archive --reason "loop retired"
EOF
  exit 1
fi

ROOT="$1"
DOMAIN="$2"
shift 2
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
OPS_SH="$SCRIPT_DIR/harness-ops"
RUNNER_SH="$SCRIPT_DIR/harness-runner"
PYTHON_CONTROL="$SCRIPT_DIR/../scripts/control.py"

if [[ ! -x "$OPS_SH" && -x "$SCRIPT_DIR/harness-ops.example.sh" ]]; then
  OPS_SH="$SCRIPT_DIR/harness-ops.example.sh"
fi
if [[ ! -x "$RUNNER_SH" && -x "$SCRIPT_DIR/harness-runner.example.sh" ]]; then
  RUNNER_SH="$SCRIPT_DIR/harness-runner.example.sh"
fi
if [[ ! -f "$PYTHON_CONTROL" && -f "$SCRIPT_DIR/control.example.py" ]]; then
  PYTHON_CONTROL="$SCRIPT_DIR/control.example.py"
fi

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
      exec "$OPS_SH" "$ROOT" "${FORMAT_ARGS[@]}" daemon "${PASSTHRU[@]}"
    elif [[ ${#FORMAT_ARGS[@]} -gt 0 ]]; then
      exec "$OPS_SH" "$ROOT" "${FORMAT_ARGS[@]}" daemon
    elif [[ ${#PASSTHRU[@]} -gt 0 ]]; then
      exec "$OPS_SH" "$ROOT" daemon "${PASSTHRU[@]}"
    else
      exec "$OPS_SH" "$ROOT" daemon
    fi
    ;;
  task)
    if [[ ${#PASSTHRU[@]} -lt 2 ]]; then
      echo "usage: $0 <ROOT> task <TASK_ID> <retry|recover|force-recover|restart|restart-from-stage|checkpoint|archive|stop|attach> [args...]" >&2
      exit 1
    fi
    TASK_ID="${PASSTHRU[0]}"
    ACTION="${PASSTHRU[1]}"
    REST=("${PASSTHRU[@]:2}")
    case "$ACTION" in
      retry|recover|force-recover)
        if [[ ${#REST[@]} -gt 0 ]]; then
          exec "$RUNNER_SH" recover "$TASK_ID" "$ROOT" "${REST[@]}"
        fi
        exec "$RUNNER_SH" recover "$TASK_ID" "$ROOT"
        ;;
      restart)
        if [[ ${#REST[@]} -gt 0 && "${REST[0]}" == "--from-stage" ]]; then
          exec python3 "$PYTHON_CONTROL" --root "$ROOT" "${FORMAT_ARGS[@]}" task "$TASK_ID" restart "${REST[@]}"
        fi
        if [[ ${#REST[@]} -gt 0 ]]; then
          exec "$RUNNER_SH" run "$TASK_ID" "$ROOT" "${REST[@]}"
        fi
        exec "$RUNNER_SH" run "$TASK_ID" "$ROOT"
        ;;
      restart-from-stage)
        if [[ ${#REST[@]} -lt 1 ]]; then
          echo "usage: $0 <ROOT> task <TASK_ID> restart-from-stage <queued|worktree_prepared|merge_queued> [--reason ...]" >&2
          exit 1
        fi
        STAGE="${REST[0]}"
        EXTRA=("${REST[@]:1}")
        if [[ ${#FORMAT_ARGS[@]} -gt 0 && ${#EXTRA[@]} -gt 0 ]]; then
          exec python3 "$PYTHON_CONTROL" --root "$ROOT" "${FORMAT_ARGS[@]}" task "$TASK_ID" restart --from-stage "$STAGE" "${EXTRA[@]}"
        elif [[ ${#FORMAT_ARGS[@]} -gt 0 ]]; then
          exec python3 "$PYTHON_CONTROL" --root "$ROOT" "${FORMAT_ARGS[@]}" task "$TASK_ID" restart --from-stage "$STAGE"
        elif [[ ${#EXTRA[@]} -gt 0 ]]; then
          exec python3 "$PYTHON_CONTROL" --root "$ROOT" task "$TASK_ID" restart --from-stage "$STAGE" "${EXTRA[@]}"
        fi
        exec python3 "$PYTHON_CONTROL" --root "$ROOT" task "$TASK_ID" restart --from-stage "$STAGE"
        ;;
      attach)
        exec "$RUNNER_SH" attach "$TASK_ID" "$ROOT"
        ;;
      checkpoint|archive|stop)
        if [[ ${#FORMAT_ARGS[@]} -gt 0 && ${#REST[@]} -gt 0 ]]; then
          exec python3 "$PYTHON_CONTROL" --root "$ROOT" "${FORMAT_ARGS[@]}" task "$TASK_ID" "$ACTION" "${REST[@]}"
        elif [[ ${#FORMAT_ARGS[@]} -gt 0 ]]; then
          exec python3 "$PYTHON_CONTROL" --root "$ROOT" "${FORMAT_ARGS[@]}" task "$TASK_ID" "$ACTION"
        elif [[ ${#REST[@]} -gt 0 ]]; then
          exec python3 "$PYTHON_CONTROL" --root "$ROOT" task "$TASK_ID" "$ACTION" "${REST[@]}"
        fi
        exec python3 "$PYTHON_CONTROL" --root "$ROOT" task "$TASK_ID" "$ACTION"
        ;;
      *)
        echo "unknown task action: $ACTION" >&2
        exit 1
        ;;
    esac
    ;;
  request)
    if [[ ${#FORMAT_ARGS[@]} -gt 0 && ${#PASSTHRU[@]} -gt 0 ]]; then
      exec python3 "$PYTHON_CONTROL" --root "$ROOT" "${FORMAT_ARGS[@]}" request "${PASSTHRU[@]}"
    elif [[ ${#FORMAT_ARGS[@]} -gt 0 ]]; then
      exec python3 "$PYTHON_CONTROL" --root "$ROOT" "${FORMAT_ARGS[@]}" request
    elif [[ ${#PASSTHRU[@]} -gt 0 ]]; then
      exec python3 "$PYTHON_CONTROL" --root "$ROOT" request "${PASSTHRU[@]}"
    fi
    exec python3 "$PYTHON_CONTROL" --root "$ROOT" request
    ;;
  project)
    if [[ ${#FORMAT_ARGS[@]} -gt 0 && ${#PASSTHRU[@]} -gt 0 ]]; then
      exec python3 "$PYTHON_CONTROL" --root "$ROOT" "${FORMAT_ARGS[@]}" project "${PASSTHRU[@]}"
    elif [[ ${#FORMAT_ARGS[@]} -gt 0 ]]; then
      exec python3 "$PYTHON_CONTROL" --root "$ROOT" "${FORMAT_ARGS[@]}" project
    elif [[ ${#PASSTHRU[@]} -gt 0 ]]; then
      exec python3 "$PYTHON_CONTROL" --root "$ROOT" project "${PASSTHRU[@]}"
    fi
    exec python3 "$PYTHON_CONTROL" --root "$ROOT" project
    ;;
  *)
    echo "unknown control domain: $DOMAIN" >&2
    exit 1
    ;;
esac
