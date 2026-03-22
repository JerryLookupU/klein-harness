#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -lt 2 ]; then
  echo "usage: $0 <PROJECT_ROOT> <PROJECT_GOAL> [STACK_HINT]" >&2
  echo "example: $0 ~/code/pomodoro-app \"建立一个简单的番茄闹钟 app\" \"React + Vite\"" >&2
  exit 1
fi

PROJECT_ROOT="$(cd "$1" && pwd)"
PROJECT_GOAL="$2"
STACK_HINT="${3:-}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
INSTALL_FULL_SH="$SCRIPT_DIR/harness-install-full.example.sh"
REPO_NAME="$(basename "$PROJECT_ROOT")"

if [ ! -d "$PROJECT_ROOT" ]; then
  echo "project root not found: $PROJECT_ROOT" >&2
  exit 1
fi

if [ ! -x "$INSTALL_FULL_SH" ]; then
  chmod +x "$INSTALL_FULL_SH"
fi

"$INSTALL_FULL_SH" "$PROJECT_ROOT"

cat <<EOF

Full harness operator toolset installed into:
  $PROJECT_ROOT/.harness

Full flow trigger:
  Open Codex in $PROJECT_ROOT and use a prompt that explicitly says:
  "使用 harness-architect skill，并进入 bootstrap 模式。"

Suggested full bootstrap prompt for Codex:

使用 harness-architect skill。
当前项目目录: $PROJECT_ROOT
项目名: $REPO_NAME
目标: $PROJECT_GOAL
技术栈提示: ${STACK_HINT:-未指定，请先探测仓库后再定}

这次不要最小流程，我要完整流程。
请进入 bootstrap 模式，为当前项目建立完整可运行的 .harness/ 协作系统，并把后续 worker / audit / operator 面一起铺好。

硬要求：
1. 先探测项目边界、源码目录、包管理器、测试命令、lint、CI、git 状态、高冲突路径。
2. 先生成 .harness/standards.md 和 .harness/verification-rules/manifest.json，再继续编排。
3. 生成 .harness/features.json、.harness/work-items.json、.harness/spec.json，先 draft，再 refinement，不要直接把粗糙任务放进 task-pool。
4. refinement 后再生成 .harness/task-pool.json、.harness/context-map.json、.harness/progress.md、.harness/session-registry.json、.harness/lineage.jsonl、.harness/audit-report.md。
5. 检查项目根 AGENTS.md：
   - 如果已有规则，保留其他工程规则。
   - 如果缺少 SOUL 段，新增。
   - 这轮把 SOUL 规范成“16岁超级天才编程少女”。
6. 让 operator plane 可直接使用，至少确认这些入口可用：
   - .harness/bin/harness-status
   - .harness/bin/harness-watch
   - .harness/bin/harness-query
   - .harness/bin/harness-dashboard
   - .harness/bin/harness-render-prompt
   - .harness/bin/harness-route-session
   - .harness/bin/harness-prepare-worktree
   - .harness/bin/harness-diff-summary
   - .harness/session-init.sh
7. 在 task-pool 里显式区分 orchestrator、worker、audit task，补齐 ownedPaths、dependsOn、verificationRuleIds、resumeStrategy、preferredResumeSessionId、worktreePath、branchName、diffBase。
8. 默认采用完整执行链：
   session-init -> program pre-worker gate -> if needed gpt-5.4 orchestration fallback -> gpt-5.3-codex worker -> audit worker -> refresh-state -> dashboard/query
9. 产出后明确告诉我：
   - 当前是 bootstrap 还是 refresh
   - 当前最高优先级 work item / task
   - 是否有 orchestration task 压在前面
   - 现在可以直接分发几个 worker task
   - 哪条命令看总览，哪条命令看 watch，哪条命令跑 session-init

After bootstrap, operator commands should look like:
  $PROJECT_ROOT/.harness/session-init.sh
  $PROJECT_ROOT/.harness/bin/harness-status $PROJECT_ROOT
  $PROJECT_ROOT/.harness/bin/harness-watch $PROJECT_ROOT 2
  $PROJECT_ROOT/.harness/bin/harness-dashboard $PROJECT_ROOT
  $PROJECT_ROOT/.harness/bin/harness-query overview $PROJECT_ROOT
  python3 $PROJECT_ROOT/.harness/scripts/refresh-state.py $PROJECT_ROOT
EOF
