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
INSTALL_TOOLS_SH="$SCRIPT_DIR/harness-install-tools.example.sh"
REPO_NAME="$(basename "$PROJECT_ROOT")"

if [ ! -d "$PROJECT_ROOT" ]; then
  echo "project root not found: $PROJECT_ROOT" >&2
  exit 1
fi

if [ ! -x "$INSTALL_TOOLS_SH" ]; then
  chmod +x "$INSTALL_TOOLS_SH"
fi

"$INSTALL_TOOLS_SH" "$PROJECT_ROOT"

cat <<EOF

Minimal harness tools installed into:
  $PROJECT_ROOT/.harness

Skill trigger note:
  The skill is triggered by your Codex prompt mentioning "klein-harness"
  or by clearly asking for bootstrap / refresh / audit / agent-entry work.

Suggested first prompt for Codex:

使用 klein-harness skill。
当前项目目录: $PROJECT_ROOT
项目名: $REPO_NAME
目标: $PROJECT_GOAL
技术栈提示: ${STACK_HINT:-未指定，请先探测仓库后再定}

请先进入 bootstrap 模式，为当前项目建立最小可用的 .harness/ 协作系统。
要求：
1. 先探测项目边界、包管理器、测试方式、主要源码目录。
2. 不要过度设计，只支持这个简单目标的最小 harness。
3. 先产出 standards.md 和 verification-rules/manifest.json，再决定是否释放 worker task。
4. 如果项目根存在 AGENTS.md，检查但不要无端重写。
5. 产出后告诉我：当前是 bootstrap 还是 refresh、最高优先级 task、是否已有可直接执行的 worker task。

After bootstrap, useful commands are:
  $PROJECT_ROOT/.harness/bin/harness-query overview $PROJECT_ROOT
  $PROJECT_ROOT/.harness/bin/harness-dashboard $PROJECT_ROOT
  python3 $PROJECT_ROOT/.harness/scripts/refresh-state.py $PROJECT_ROOT
EOF
