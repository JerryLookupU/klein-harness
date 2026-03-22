#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF' >&2
usage: harness-kick [options] <PROJECT_GOAL> [STACK_HINT] [PROJECT_ROOT]

options:
  --manual, --no-bootstrap  only install .harness and write bootstrap prompt
  --auto-bootstrap          force running `codex exec` bootstrap after install
  --replace-bootstrap       stop an existing bootstrap tmux session for this project before starting a new one
  --no-session-init         skip automatic session-init after bootstrap
  --no-daemon               do not start the runner daemon after bootstrap
  --context <PATH>          attach an extra context file or directory
  --prd <PATH>              alias of --context
  --model <MODEL>           pass model to `codex exec`
  --concurrency <N>         write a worker parallelism preference into the prompt
  --daemon                  explicitly enable the runner daemon after bootstrap
  --daemon-interval <N>     runner daemon tick interval in seconds (default: 60)
  -h, --help                show this help

examples:
  harness-kick "我要创建一个番茄时钟项目" "React + Vite"
  harness-kick --context docs/notes.md "帮我简单分析一下我的代码，给一个markdown 分析报告"
  harness-kick --prd docs/prd.md "帮我简单分析一下我的代码，给一个markdown 分析报告"
  harness-kick --manual "帮我简单分析一下我的代码，给一个markdown 分析报告"
  harness-kick --model gpt-5 --concurrency 4 "实现完整 bootstrap" "" /path/to/project
EOF
}

POSITIONAL=()
AUTO_BOOTSTRAP=1
RUN_SESSION_INIT=1
MODEL=""
CONCURRENCY=""
RUN_DAEMON=1
DAEMON_INTERVAL=60
REPLACE_BOOTSTRAP=0
CONTEXT_INPUTS=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --manual|--no-bootstrap)
      AUTO_BOOTSTRAP=0
      shift
      ;;
    --auto-bootstrap)
      AUTO_BOOTSTRAP=1
      shift
      ;;
    --replace-bootstrap)
      REPLACE_BOOTSTRAP=1
      shift
      ;;
    --no-session-init)
      RUN_SESSION_INIT=0
      shift
      ;;
    --no-daemon)
      RUN_DAEMON=0
      shift
      ;;
    --context|--prd)
      if [[ $# -lt 2 ]]; then
        echo "missing value for $1" >&2
        usage
        exit 1
      fi
      CONTEXT_INPUTS+=("$2")
      shift 2
      ;;
    --model)
      if [[ $# -lt 2 ]]; then
        echo "missing value for --model" >&2
        usage
        exit 1
      fi
      MODEL="$2"
      shift 2
      ;;
    --concurrency)
      if [[ $# -lt 2 ]]; then
        echo "missing value for --concurrency" >&2
        usage
        exit 1
      fi
      CONCURRENCY="$2"
      shift 2
      ;;
    --daemon)
      RUN_DAEMON=1
      shift
      ;;
    --daemon-interval)
      if [[ $# -lt 2 ]]; then
        echo "missing value for --daemon-interval" >&2
        usage
        exit 1
      fi
      DAEMON_INTERVAL="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    --)
      shift
      while [[ $# -gt 0 ]]; do
        POSITIONAL+=("$1")
        shift
      done
      ;;
    -*)
      echo "unknown option: $1" >&2
      usage
      exit 1
      ;;
    *)
      POSITIONAL+=("$1")
      shift
      ;;
  esac
done

if [[ ${#POSITIONAL[@]} -lt 1 ]]; then
  usage
  exit 1
fi

if [[ -n "$CONCURRENCY" && ! "$CONCURRENCY" =~ ^[1-9][0-9]*$ ]]; then
  echo "--concurrency must be a positive integer" >&2
  exit 1
fi

if [[ ! "$DAEMON_INTERVAL" =~ ^[1-9][0-9]*$ ]]; then
  echo "--daemon-interval must be a positive integer" >&2
  exit 1
fi

PROJECT_GOAL="${POSITIONAL[0]}"
STACK_HINT="${POSITIONAL[1]:-}"
PROJECT_ROOT_INPUT="${POSITIONAL[2]:-$(pwd)}"
if [[ "$PROJECT_ROOT_INPUT" = /* ]]; then
  PROJECT_ROOT="$PROJECT_ROOT_INPUT"
else
  PROJECT_ROOT="$(pwd)/$PROJECT_ROOT_INPUT"
fi
mkdir -p "$PROJECT_ROOT"
PROJECT_ROOT="$(cd "$PROJECT_ROOT" && pwd)"
CONTEXT_PATHS=()
CONTEXT_PROMPT_BLOCK=""
ADD_DIR_ARGS=()
LAUNCHED_BOOTSTRAP_TMUX_SESSION=""

resolve_context_path() {
  local input="$1"
  if [[ "$input" = /* ]]; then
    printf '%s\n' "$input"
  else
    printf '%s/%s\n' "$(cd "$PROJECT_ROOT" && cd "$(dirname "$input")" && pwd)" "$(basename "$input")"
  fi
}

append_unique_add_dir() {
  local candidate="$1"
  local existing
  for existing in "${ADD_DIR_ARGS[@]:-}"; do
    if [[ "$existing" == "$candidate" ]]; then
      return 0
    fi
  done
  ADD_DIR_ARGS+=("$candidate")
}

make_bootstrap_tmux_session_name() {
  local base_name=""
  local ts=""

  base_name="$(basename "$PROJECT_ROOT" | tr -cs '[:alnum:]' '-' | sed 's/^-*//; s/-*$//')"
  [[ -n "$base_name" ]] || base_name="project"
  base_name="${base_name:0:24}"
  ts="$(date +%m%d-%H%M%S)"
  printf 'hk-bootstrap-%s-%s\n' "$base_name" "$ts"
}

make_runner_daemon_tmux_session_name() {
  local base_name=""
  local ts=""

  base_name="$(basename "$PROJECT_ROOT" | tr -cs '[:alnum:]' '-' | sed 's/^-*//; s/-*$//')"
  [[ -n "$base_name" ]] || base_name="project"
  base_name="${base_name:0:24}"
  ts="$(date +%m%d-%H%M%S)"
  printf 'hr-daemon-%s-%s\n' "$base_name" "$ts"
}

resolve_bootstrap_tmux_session() {
  local session_name=""

  if [[ -n "$LAUNCHED_BOOTSTRAP_TMUX_SESSION" ]]; then
    session_name="$LAUNCHED_BOOTSTRAP_TMUX_SESSION"
  elif [[ -f "$BOOTSTRAP_SESSION_PATH" ]]; then
    session_name="$(head -n 1 "$BOOTSTRAP_SESSION_PATH" 2>/dev/null || true)"
  fi

  if [[ -z "$session_name" ]] || ! command -v tmux >/dev/null 2>&1; then
    return 1
  fi

  if tmux has-session -t "$session_name" 2>/dev/null; then
    printf '%s\n' "$session_name"
    return 0
  fi

  return 1
}

bootstrap_tmux_session_is_running() {
  local session_name="$1"
  local pane_dead=""

  [[ -n "$session_name" ]] || return 1
  command -v tmux >/dev/null 2>&1 || return 1
  tmux has-session -t "$session_name" 2>/dev/null || return 1

  pane_dead="$(tmux list-panes -t "$session_name" -F '#{pane_dead}' 2>/dev/null | head -n 1 || true)"
  [[ "$pane_dead" == "0" ]]
}

resolve_running_bootstrap_tmux_session() {
  local session_name=""

  session_name="$(resolve_bootstrap_tmux_session || true)"
  [[ -n "$session_name" ]] || return 1

  if bootstrap_tmux_session_is_running "$session_name"; then
    printf '%s\n' "$session_name"
    return 0
  fi

  return 1
}

resolve_runner_daemon_tmux_session() {
  local session_name=""

  if [[ -f "$RUNNER_DAEMON_SESSION_PATH" ]]; then
    session_name="$(head -n 1 "$RUNNER_DAEMON_SESSION_PATH" 2>/dev/null || true)"
  fi

  if [[ -z "$session_name" ]] || ! command -v tmux >/dev/null 2>&1; then
    return 1
  fi

  if tmux has-session -t "$session_name" 2>/dev/null; then
    printf '%s\n' "$session_name"
    return 0
  fi

  return 1
}

update_project_meta() {
  local lifecycle="$1"
  local bootstrap_status="$2"
  local bootstrap_session="${3:-}"

  python3 - "$PROJECT_META_PATH" "$PROJECT_ROOT" "$lifecycle" "$bootstrap_status" "$bootstrap_session" <<'PY'
import json
import sys
from datetime import datetime, timezone
from pathlib import Path

meta_path = Path(sys.argv[1])
project_root = sys.argv[2]
lifecycle = sys.argv[3]
bootstrap_status = sys.argv[4]
bootstrap_session = sys.argv[5]
timestamp = datetime.now(timezone.utc).astimezone().isoformat(timespec="seconds")

data = {}
if meta_path.exists():
    data = json.loads(meta_path.read_text())

data.setdefault("schemaVersion", "1.0")
data.setdefault("generator", "klein-harness")
data.setdefault("requestQueueEnabled", True)
data["generatedAt"] = timestamp
data["projectRoot"] = project_root
data["lifecycle"] = lifecycle
data["bootstrapStatus"] = bootstrap_status
data["lastBootstrapAt"] = timestamp
data["bootstrapSession"] = bootstrap_session or None

meta_path.parent.mkdir(parents=True, exist_ok=True)
meta_path.write_text(json.dumps(data, ensure_ascii=False, indent=2) + "\n")
PY
}

stop_bootstrap_tmux_session() {
  local session_name=""
  session_name="$(resolve_bootstrap_tmux_session || true)"
  [[ -n "$session_name" ]] || return 1
  command -v tmux >/dev/null 2>&1 || return 1
  tmux kill-session -t "$session_name"
  rm -f "$BOOTSTRAP_SESSION_PATH"
  return 0
}

launch_runner_daemon_in_tmux() {
  mkdir -p "$PROJECT_ROOT/.harness/state"
  : > "$RUNNER_DAEMON_STDOUT_PATH"
  if ! "$PROJECT_ROOT/.harness/bin/harness-runner" daemon "$PROJECT_ROOT" --interval "$DAEMON_INTERVAL" --replace >/dev/null; then
    return 1
  fi
  if [[ -f "$RUNNER_DAEMON_SESSION_PATH" ]]; then
    return 0
  fi
  return 0
}


launch_bootstrap_in_tmux() {
  local session_name=""
  local prompt_literal=""
  local result_literal=""
  local stdout_literal=""
  local project_literal=""
  local project_meta_literal=""
  local session_path_literal=""
  local runner_path_literal=""
  local refresh_literal=""
  local session_init_literal=""
  local status_literal=""
  local runner_literal=""
  local runner_daemon_session_literal=""
  local runner_daemon_log_literal=""
  local codex_cmd_literal=""
  local session_name_literal=""

  session_name="$(make_bootstrap_tmux_session_name)"
  prompt_literal="$(printf '%q' "$PROMPT_PATH")"
  result_literal="$(printf '%q' "$BOOTSTRAP_RESULT_PATH")"
  stdout_literal="$(printf '%q' "$BOOTSTRAP_STDOUT_PATH")"
  project_literal="$(printf '%q' "$PROJECT_ROOT")"
  project_meta_literal="$(printf '%q' "$PROJECT_META_PATH")"
  session_path_literal="$(printf '%q' "$BOOTSTRAP_SESSION_PATH")"
  runner_path_literal="$(printf '%q' "$BOOTSTRAP_RUNNER_PATH")"
  refresh_literal="$(printf '%q' "$PROJECT_ROOT/.harness/scripts/refresh-state.py")"
  session_init_literal="$(printf '%q' "$PROJECT_ROOT/.harness/session-init.sh")"
  status_literal="$(printf '%q' "$PROJECT_ROOT/.harness/bin/harness-status")"
  runner_literal="$(printf '%q' "$PROJECT_ROOT/.harness/bin/harness-runner")"
  runner_daemon_session_literal="$(printf '%q' "$RUNNER_DAEMON_SESSION_PATH")"
  runner_daemon_log_literal="$(printf '%q' "$RUNNER_DAEMON_STDOUT_PATH")"
  codex_cmd_literal="$(printf '%q ' "${CODEX_CMD[@]}")"
  session_name_literal="$(printf '%q' "$session_name")"

  mkdir -p "$PROJECT_ROOT/.harness/state"
  rm -f "$BOOTSTRAP_RESULT_PATH"

  cat > "$BOOTSTRAP_RUNNER_PATH" <<EOF
#!/usr/bin/env bash
set -euo pipefail

PROJECT_ROOT=$project_literal
PROJECT_META_PATH=$project_meta_literal
BOOTSTRAP_SESSION_NAME=$session_name_literal
BOOTSTRAP_SESSION_PATH=$session_path_literal
BOOTSTRAP_RUNNER_PATH=$runner_path_literal
PROMPT_PATH=$prompt_literal
BOOTSTRAP_RESULT_PATH=$result_literal
BOOTSTRAP_STDOUT_PATH=$stdout_literal
RUN_SESSION_INIT=$RUN_SESSION_INIT
RUN_DAEMON=$RUN_DAEMON
DAEMON_INTERVAL=$DAEMON_INTERVAL
CODEX_CMD=($codex_cmd_literal)
RUNNER_BIN=$runner_literal
RUNNER_DAEMON_SESSION_PATH=$runner_daemon_session_literal
RUNNER_DAEMON_LOG_PATH=$runner_daemon_log_literal

update_project_meta() {
  python3 - "\$PROJECT_META_PATH" "\$PROJECT_ROOT" "\$1" "\$2" "\${3:-}" <<'PY'
import json
import sys
from datetime import datetime, timezone
from pathlib import Path

meta_path = Path(sys.argv[1])
project_root = sys.argv[2]
lifecycle = sys.argv[3]
bootstrap_status = sys.argv[4]
bootstrap_session = sys.argv[5]
timestamp = datetime.now(timezone.utc).astimezone().isoformat(timespec="seconds")

data = {}
if meta_path.exists():
    data = json.loads(meta_path.read_text())

data.setdefault("schemaVersion", "1.0")
data.setdefault("generator", "klein-harness")
data.setdefault("requestQueueEnabled", True)
data["generatedAt"] = timestamp
data["projectRoot"] = project_root
data["lifecycle"] = lifecycle
data["bootstrapStatus"] = bootstrap_status
data["lastBootstrapAt"] = timestamp
data["bootstrapSession"] = bootstrap_session or None

meta_path.parent.mkdir(parents=True, exist_ok=True)
meta_path.write_text(json.dumps(data, ensure_ascii=False, indent=2) + "\n")
PY
}

cleanup_bootstrap_artifacts() {
  rm -f "\$BOOTSTRAP_SESSION_PATH" "\$BOOTSTRAP_RUNNER_PATH"
}

trap cleanup_bootstrap_artifacts EXIT

cd "\$PROJECT_ROOT"
exec > >(tee -a "\$BOOTSTRAP_STDOUT_PATH") 2>&1

echo "[bootstrap] started at \$(date '+%Y-%m-%d %H:%M:%S')"
update_project_meta "bootstrapping" "running" "\$BOOTSTRAP_SESSION_NAME"

if "\${CODEX_CMD[@]}" - < "\$PROMPT_PATH"; then
  if [[ ! -s "\$BOOTSTRAP_RESULT_PATH" ]]; then
    echo "[bootstrap] codex exec exited before writing \$BOOTSTRAP_RESULT_PATH"
    update_project_meta "initialized" "failed" ""
    exit 1
  fi
  echo "[bootstrap] bootstrap result saved to \$BOOTSTRAP_RESULT_PATH"
else
  status=\$?
  echo "[bootstrap] codex exec failed with exit code \$status"
  update_project_meta "initialized" "failed" ""
  exit \$status
fi

if [[ -f $refresh_literal ]]; then
  echo "[bootstrap] refreshing hot state..."
  python3 $refresh_literal "\$PROJECT_ROOT" || echo "[bootstrap] refresh-state failed"
fi

if [[ "\$RUN_SESSION_INIT" -eq 1 && -x $session_init_literal ]]; then
  echo "[bootstrap] running session-init..."
  $session_init_literal || echo "[bootstrap] session-init reported drift or incomplete bootstrap"
fi

if [[ -x $status_literal ]]; then
  echo "[bootstrap] current status:"
  $status_literal "\$PROJECT_ROOT" || true
fi

if [[ "\$RUN_DAEMON" -eq 1 && -x "\$RUNNER_BIN" ]]; then
  echo "[bootstrap] launching runner daemon..."
  if "\$RUNNER_BIN" daemon "\$PROJECT_ROOT" --interval "\$DAEMON_INTERVAL" --replace >/dev/null; then
    echo "[bootstrap] runner daemon interval: \${DAEMON_INTERVAL}s"
    if [[ -f "\$RUNNER_DAEMON_SESSION_PATH" ]]; then
      echo "[bootstrap] runner daemon session: \$(head -n 1 "\$RUNNER_DAEMON_SESSION_PATH" 2>/dev/null || true)"
    fi
    echo "[bootstrap] runner daemon log: \$RUNNER_DAEMON_LOG_PATH"
  else
    echo "[bootstrap] failed to launch runner daemon"
  fi
fi

update_project_meta "active" "ready" ""
echo "[bootstrap] finished at \$(date '+%Y-%m-%d %H:%M:%S')"
EOF

  chmod +x "$BOOTSTRAP_RUNNER_PATH"

  if ! tmux new-session -d -s "$session_name" "bash $(printf '%q' "$BOOTSTRAP_RUNNER_PATH")"; then
    return 1
  fi

  printf '%s\n' "$session_name" > "$BOOTSTRAP_SESSION_PATH"
  LAUNCHED_BOOTSTRAP_TMUX_SESSION="$session_name"
  return 0
}

if [[ ${#CONTEXT_INPUTS[@]} -gt 0 ]]; then
  CONTEXT_PROMPT_BLOCK=$'附加上下文路径：\n'
  for context_input in "${CONTEXT_INPUTS[@]}"; do
    context_path="$(resolve_context_path "$context_input")"
    if [[ ! -e "$context_path" ]]; then
      echo "context path not found: $context_input" >&2
      exit 1
    fi
    CONTEXT_PATHS+=("$context_path")
    CONTEXT_PROMPT_BLOCK+="- $context_path"$'\n'
    case "$context_path" in
      "$PROJECT_ROOT"/*) ;;
      *)
        if [[ -d "$context_path" ]]; then
          append_unique_add_dir "$context_path"
        else
          append_unique_add_dir "$(dirname "$context_path")"
        fi
        ;;
    esac
  done
  CONTEXT_PROMPT_BLOCK+=$'\n补充要求：\n- 在探测仓库和编排任务前，先读取这些附加上下文。\n- 如果上下文与仓库现状冲突，先在分析里明确冲突点，再决定 bootstrap / refinement。\n'
fi

CODEX_BASE="${CODEX_HOME:-$HOME/.codex}"
SKILL_DIR="$CODEX_BASE/skills/klein-harness"
EXAMPLES_DIR="$SKILL_DIR/examples"
INSTALL_FULL_SH="$EXAMPLES_DIR/harness-install-full.example.sh"
PROMPT_PATH="$PROJECT_ROOT/.harness/bootstrap-request.md"
BOOTSTRAP_RESULT_PATH="$PROJECT_ROOT/.harness/bootstrap-result.md"
BOOTSTRAP_STDOUT_PATH="$PROJECT_ROOT/.harness/state/bootstrap-output.log"
BOOTSTRAP_RUNNER_PATH="$PROJECT_ROOT/.harness/state/bootstrap-runner.sh"
BOOTSTRAP_SESSION_PATH="$PROJECT_ROOT/.harness/state/bootstrap-tmux-session.txt"
RUNNER_DAEMON_SESSION_PATH="$PROJECT_ROOT/.harness/state/runner-daemon-tmux-session.txt"
RUNNER_DAEMON_STDOUT_PATH="$PROJECT_ROOT/.harness/state/runner-daemon.log"
PROJECT_META_PATH="$PROJECT_ROOT/.harness/project-meta.json"

if [[ ! -d "$SKILL_DIR" ]]; then
  echo "klein-harness skill is not installed: $SKILL_DIR" >&2
  echo "run: ./install.sh --force" >&2
  exit 1
fi

if [[ ! -x "$INSTALL_FULL_SH" ]]; then
  chmod +x "$INSTALL_FULL_SH"
fi

bash "$INSTALL_FULL_SH" "$PROJECT_ROOT"
mkdir -p "$PROJECT_ROOT/.harness"
update_project_meta "initialized" "not_started"

CONCURRENCY_PROMPT_LINE=""
CONCURRENCY_HARD_REQUIREMENT=""
MODEL_ARG_TEXT=""
ADD_DIR_ARG_TEXT=""

if [[ -n "$CONCURRENCY" ]]; then
  CONCURRENCY_PROMPT_LINE="并发偏好: 最多 ${CONCURRENCY} 个 worker 并发（仅在依赖图允许时）。"
  CONCURRENCY_HARD_REQUIREMENT="10. 如果 bootstrap 后存在可并发 worker task，按最多 ${CONCURRENCY} 个 worker 的节奏给出建议派发批次，并在输出里说明并发上限来自用户参数。"
fi

if [[ -n "$MODEL" ]]; then
  MODEL_ARG_TEXT=" -m $MODEL"
fi

if [[ ${#ADD_DIR_ARGS[@]} -gt 0 ]]; then
  for add_dir in "${ADD_DIR_ARGS[@]}"; do
    ADD_DIR_ARG_TEXT+=" --add-dir \"$add_dir\""
  done
fi

cat > "$PROMPT_PATH" <<EOF
使用 klein-harness skill。
当前项目目录: $PROJECT_ROOT
项目名: $(basename "$PROJECT_ROOT")
目标: $PROJECT_GOAL
技术栈提示: ${STACK_HINT:-未指定，请先探测仓库后再定}
${CONCURRENCY_PROMPT_LINE}
${CONTEXT_PROMPT_BLOCK}

这次不要最小流程，我要完整流程。
请进入 bootstrap 模式，为当前项目建立完整可运行的 .harness/ 协作系统，并把后续 worker / audit / operator 面一起铺好。

硬要求：
1. 先探测项目边界、源码目录、包管理器、测试命令、lint、CI、git 状态、高冲突路径。
2. 先读取用户指定的附加上下文（如果存在 --context / --prd），再做仓库探测与分析。
3. 先生成 .harness/standards.md 和 .harness/verification-rules/manifest.json，再继续编排。
4. 生成 .harness/features.json、.harness/work-items.json、.harness/spec.json，先 draft，再 refinement，不要直接把粗糙任务放进 task-pool。
5. refinement 后再生成 .harness/task-pool.json、.harness/context-map.json、.harness/progress.md、.harness/session-registry.json、.harness/lineage.jsonl、.harness/audit-report.md。
6. 检查项目根 AGENTS.md：
   - 如果已有规则，保留其他工程规则。
   - 如果缺少 SOUL 段，新增。
   - SOUL 段保持工程中性，只描述协作风格和执行约束。
   - 除非用户明确要求，不要强加具体人格模板。
7. 让 operator plane 可直接使用，至少确认这些入口可用：
   - .harness/bin/harness-status
   - .harness/bin/harness-watch
   - .harness/bin/harness-query
   - .harness/bin/harness-dashboard
   - .harness/bin/harness-render-prompt
   - .harness/bin/harness-route-session
   - .harness/bin/harness-prepare-worktree
   - .harness/bin/harness-diff-summary
   - .harness/bin/harness-verify-task
   - .harness/session-init.sh
8. 在 task-pool 里显式区分 orchestrator、worker、audit task，补齐 ownedPaths、dependsOn、verificationRuleIds、resumeStrategy、preferredResumeSessionId、worktreePath、branchName、diffBase。
9. 默认采用完整执行链：
   session-init -> program pre-worker gate -> if needed gpt-5.4 orchestration fallback -> gpt-5.3-codex worker -> audit worker -> refresh-state -> dashboard/query
10. 产出后明确告诉我：
   - 当前是 bootstrap 还是 refresh
   - 当前最高优先级 work item / task
   - 是否有 orchestration task 压在前面
   - 现在可以直接分发几个 worker task
   - 哪条命令看总览，哪条命令看 watch，哪条命令跑 session-init
${CONCURRENCY_HARD_REQUIREMENT}
EOF

if command -v pbcopy >/dev/null 2>&1; then
  pbcopy < "$PROMPT_PATH"
fi

echo
echo "bootstrap prompt saved to $PROMPT_PATH"
if command -v pbcopy >/dev/null 2>&1; then
  echo "bootstrap prompt copied to clipboard"
fi
if [[ ${#CONTEXT_PATHS[@]} -gt 0 ]]; then
  echo "attached context paths:"
  for context_path in "${CONTEXT_PATHS[@]}"; do
    echo "  - $context_path"
  done
fi

print_commands() {
  local bootstrap_tmux_session=""

  bootstrap_tmux_session="$(resolve_bootstrap_tmux_session || true)"

  cat <<EOF

Operator commands:
  "$PROJECT_ROOT/.harness/bin/harness-status" "$PROJECT_ROOT"
  "$PROJECT_ROOT/.harness/bin/harness-watch" "$PROJECT_ROOT" 2
  "$PROJECT_ROOT/.harness/bin/harness-query" overview "$PROJECT_ROOT" --text
  "$PROJECT_ROOT/.harness/bin/harness-query" current "$PROJECT_ROOT" --text
  "$PROJECT_ROOT/.harness/bin/harness-dashboard" "$PROJECT_ROOT"
  "$PROJECT_ROOT/.harness/bin/harness-report" "$PROJECT_ROOT"
  "$PROJECT_ROOT/.harness/bin/harness-submit" "$PROJECT_ROOT" --kind status --goal "汇报当前进度"
  "$PROJECT_ROOT/.harness/bin/harness-runner" tick "$PROJECT_ROOT"
  "$PROJECT_ROOT/.harness/bin/harness-runner" list "$PROJECT_ROOT"
  "$PROJECT_ROOT/.harness/bin/harness-runner" attach <TASK_ID> "$PROJECT_ROOT"
  "$PROJECT_ROOT/.harness/bin/harness-runner" recover <TASK_ID> "$PROJECT_ROOT"
  "$PROJECT_ROOT/.harness/bin/harness-verify-task" <TASK_ID> "$PROJECT_ROOT" --write-back
  python3 "$PROJECT_ROOT/.harness/scripts/refresh-state.py" "$PROJECT_ROOT"
  "$PROJECT_ROOT/.harness/session-init.sh"
  sed -n '1,160p' "$BOOTSTRAP_RESULT_PATH"
  tail -n 60 "$BOOTSTRAP_STDOUT_PATH"
  tmux ls
EOF
  if [[ -n "$bootstrap_tmux_session" ]]; then
    cat <<EOF
  tmux attach -t "$bootstrap_tmux_session"
  tmux capture-pane -pt "$bootstrap_tmux_session"
EOF
  else
    cat <<'EOF'
  tmux attach -t <session>
  tmux capture-pane -pt <session>
EOF
  fi
cat <<EOF

Useful reruns:
  harness-kick --manual "$PROJECT_GOAL" "${STACK_HINT}" "$PROJECT_ROOT"
EOF
  if [[ ${#CONTEXT_PATHS[@]} -gt 0 ]]; then
    for context_path in "${CONTEXT_PATHS[@]}"; do
      cat <<EOF
  harness-kick --context "$context_path" "$PROJECT_GOAL" "${STACK_HINT}" "$PROJECT_ROOT"
EOF
    done
  fi
  if [[ -n "$CONCURRENCY" ]]; then
    cat <<EOF
  harness-kick --concurrency $CONCURRENCY "$PROJECT_GOAL" "${STACK_HINT}" "$PROJECT_ROOT"
EOF
  fi
  if [[ -n "$MODEL" ]]; then
    cat <<EOF
  harness-kick --model $MODEL "$PROJECT_GOAL" "${STACK_HINT}" "$PROJECT_ROOT"
EOF
  fi
}

bootstrap_ready() {
  [[ -f "$PROJECT_ROOT/.harness/progress.md" ]] \
    && [[ -f "$PROJECT_ROOT/.harness/task-pool.json" ]] \
    && [[ -f "$PROJECT_ROOT/.harness/work-items.json" ]] \
    && [[ -f "$PROJECT_ROOT/.harness/spec.json" ]]
}

BOOTSTRAP_ATTEMPTED=0
BOOTSTRAP_SUCCEEDED=0
BOOTSTRAP_DISPATCHED=0

if [[ "$AUTO_BOOTSTRAP" -eq 1 ]]; then
  if ! command -v codex >/dev/null 2>&1; then
    echo
    echo "codex CLI not found in PATH; falling back to manual bootstrap"
    AUTO_BOOTSTRAP=0
  fi
fi

if [[ "$AUTO_BOOTSTRAP" -eq 1 ]]; then
  existing_bootstrap_session="$(resolve_running_bootstrap_tmux_session || true)"
  if [[ -n "$existing_bootstrap_session" ]]; then
    if [[ "$REPLACE_BOOTSTRAP" -eq 1 ]]; then
      echo
      echo "replacing existing bootstrap session: $existing_bootstrap_session"
      stop_bootstrap_tmux_session || true
      update_project_meta "initialized" "replaced"
    else
      echo
      echo "existing bootstrap session is still active for this project: $existing_bootstrap_session" >&2
      echo "refusing to launch another bootstrap against the same .harness control surface" >&2
      print_commands
      cat <<EOF

To replace the active bootstrap session:
  harness-kick --replace-bootstrap "$PROJECT_GOAL" "${STACK_HINT}" "$PROJECT_ROOT"
EOF
      exit 1
    fi
  fi

  BOOTSTRAP_ATTEMPTED=1
  update_project_meta "bootstrapping" "queued"
  echo
  echo "starting automatic bootstrap in a detached tmux session..."

  CODEX_CMD=(codex exec --yolo --skip-git-repo-check -C "$PROJECT_ROOT" -o "$BOOTSTRAP_RESULT_PATH")
  if [[ -n "$MODEL" ]]; then
    CODEX_CMD+=(-m "$MODEL")
  fi
  if [[ ${#ADD_DIR_ARGS[@]} -gt 0 ]]; then
    for add_dir in "${ADD_DIR_ARGS[@]}"; do
      CODEX_CMD+=(--add-dir "$add_dir")
    done
  fi

  if command -v tmux >/dev/null 2>&1 && launch_bootstrap_in_tmux; then
    BOOTSTRAP_DISPATCHED=1
    update_project_meta "bootstrapping" "running" "$LAUNCHED_BOOTSTRAP_TMUX_SESSION"
    echo
    echo "bootstrap is running in the background."
    echo "tmux session: $LAUNCHED_BOOTSTRAP_TMUX_SESSION"
    echo "bootstrap log: $BOOTSTRAP_STDOUT_PATH"
    if [[ "$RUN_DAEMON" -eq 1 ]]; then
      echo "runner daemon: will auto-start after bootstrap completes"
      echo "runner daemon interval: ${DAEMON_INTERVAL}s"
      echo "runner daemon log: $RUNNER_DAEMON_STDOUT_PATH"
    fi
  else
    echo
    echo "background dispatch failed; falling back to foreground bootstrap"
    update_project_meta "bootstrapping" "running"
    rm -f "$BOOTSTRAP_RESULT_PATH"
    if "${CODEX_CMD[@]}" - < "$PROMPT_PATH"; then
      if [[ -s "$BOOTSTRAP_RESULT_PATH" ]]; then
        BOOTSTRAP_SUCCEEDED=1
        update_project_meta "active" "ready"
        echo
        echo "bootstrap response saved to $BOOTSTRAP_RESULT_PATH"
      else
        update_project_meta "initialized" "failed"
        echo
        echo "automatic bootstrap was interrupted or exited before writing $BOOTSTRAP_RESULT_PATH"
        echo "prompt is ready for manual retry"
      fi
    else
      update_project_meta "initialized" "failed"
      echo
      echo "automatic bootstrap failed; prompt is ready for manual retry"
    fi
  fi
fi

if [[ "$BOOTSTRAP_SUCCEEDED" -eq 1 ]] && bootstrap_ready; then
  if [[ -f "$PROJECT_ROOT/.harness/scripts/refresh-state.py" ]]; then
    echo
    echo "refreshing hot state..."
    if ! python3 "$PROJECT_ROOT/.harness/scripts/refresh-state.py" "$PROJECT_ROOT"; then
      echo "refresh-state failed; continuing" >&2
    fi
  fi

  if [[ "$RUN_SESSION_INIT" -eq 1 && -x "$PROJECT_ROOT/.harness/session-init.sh" ]]; then
    echo
    echo "running session-init..."
    if ! "$PROJECT_ROOT/.harness/session-init.sh"; then
      echo "session-init reported drift or incomplete bootstrap; inspect the commands below" >&2
    fi
  fi

  if [[ -x "$PROJECT_ROOT/.harness/bin/harness-status" ]]; then
    echo
    "$PROJECT_ROOT/.harness/bin/harness-status" "$PROJECT_ROOT" || true
  fi

  if [[ "$RUN_DAEMON" -eq 1 ]]; then
    echo
    echo "launching runner daemon tmux session..."
    if launch_runner_daemon_in_tmux; then
      echo "runner daemon session: $(head -n 1 "$RUNNER_DAEMON_SESSION_PATH" 2>/dev/null || true)"
      echo "runner daemon interval: ${DAEMON_INTERVAL}s"
      echo "runner daemon log: $RUNNER_DAEMON_STDOUT_PATH"
    else
      echo "failed to launch runner daemon session" >&2
    fi
  fi
elif [[ "$BOOTSTRAP_ATTEMPTED" -eq 1 && "$BOOTSTRAP_SUCCEEDED" -eq 1 ]]; then
  echo
  echo "bootstrap finished but expected harness state files were not generated; skipping refresh-state and session-init"
fi

print_commands

if [[ "$AUTO_BOOTSTRAP" -eq 0 || ( "$BOOTSTRAP_SUCCEEDED" -eq 0 && "$BOOTSTRAP_DISPATCHED" -eq 0 ) ]]; then
  update_project_meta "initialized" "prompt_ready"
  cat <<EOF

Manual bootstrap:
  codex exec --yolo --skip-git-repo-check -C "$PROJECT_ROOT"$MODEL_ARG_TEXT$ADD_DIR_ARG_TEXT -o "$BOOTSTRAP_RESULT_PATH" - < "$PROMPT_PATH"
EOF
fi
