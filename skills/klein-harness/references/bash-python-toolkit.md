# Bash + Python Toolkit

在以下情况读取本文件：

- 用户明确说“希望用 bash 就能管理”
- 你要给 harness 设计可复用脚本入口
- 你要处理谱系校验、冲突判断、claim、replan、stop、status、metrics

核心建议只有一句：

> bash 做入口，Python 做结构和规则。

不要反过来：

- 不要让用户日常管理必须手写 Python 命令
- 也不要让纯 bash 去硬扛树遍历、JSON 改写、冲突图判断

## 推荐目录

```text
bin/
  harness-status
  harness-query
  harness-dashboard
  harness-install-tools
  harness-watch
  harness-metrics
  harness-render-prompt
  harness-claim-next
  harness-prepare-worktree
  harness-diff-summary
  harness-request-replan
  harness-request-stop
  harness-checkpoint
  harness-tree-check
  harness-tree-view
  harness-resolve-orchestrator

.harness/scripts/
  lib.py
  status.py
  query.py
  refresh_state.py
  metrics.py
  render_prompt.py
  prepare_worktree.py
  diff_summary.py
  claim_next.py
  request_replan.py
  request_stop.py
  checkpoint.py
  tree_check.py
  tree_view.py
  resolve_orchestrator.py
```

## 设计分工

### bash 负责

- 给人类一个稳定入口
- 做路径定位
- 透传参数
- 输出更友好的帮助信息
- 组合多个 Python 动作

### Python 负责

- 读写 JSON
- 遍历 task 树
- 检查 `parent/child/lineagePath`
- 计算冲突
- 生成 replan / stop request
- 计算 metrics
- 选择下一个可执行 leaf task

## 最小脚本集

如果只做第一轮，不要贪多。先做这 5 个：

### 1. `harness-status`

输出：

- 当前 mode
- 当前 focus
- 当前 active task
- 当前一句话 `summary`
- 当前 blocker
- 当前 risk / watch / gap 统计
- 当前 active worker / orchestrator 数
- 优先读 `.harness/state/current.json` 和 `.harness/state/runtime.json`

### 1.5 `harness-query`

职责：

- 给其他工具稳定返回机器可读状态
- 避免每个外部工具自己解析 `progress.md`
- 优先读 `.harness/state/*.json`
- 支持至少这几类查询：
  - `overview`
  - `progress`
  - `current`
  - `blueprint`
  - `task`

### 1.6 `harness-dashboard`

职责：

- 用一个 CLI 面板聚合：
  - `overview`
  - `current`
  - `progress`
  - `blueprint`
  - 可选 `task`
- 支持自动刷新
- 优先给人读，而不是给程序读

### 1.7 `harness-install-tools`

职责：

- 把 skill 里的 canonical 模板复制到项目 `.harness`
- 维护 `tooling-manifest.json`
- 支持最小安装集和增量安装

### 2. `harness-watch`

职责：

- 每 1-2 秒刷新 `harness-status`
- 给 operator 一个不需要读 JSON 的实时面板

### 2.5 `refresh-state`

职责：

- 刷新 `state/current.json`
- 刷新 `state/runtime.json`
- 刷新 `state/blueprint-index.json`
- 让 query / dashboard 优先走热路径

### 3. `harness-claim-next`

职责：

- 找下一个安全可执行的 `leaf task`
- 避免和 active sibling / ancestor / descendant 冲突
- 避免碰共享账本 merge gate

### 4. `harness-request-replan`

职责：

- 生成局部 replan request
- 限定 scope 为 `self / subtree / parent`
- 可附带 `suggestedStopTaskIds`

### 5. `harness-tree-check`

职责：

- 检查深度是否超标
- 检查 parent / child 一致性
- 检查 `lineagePath` 是否匹配
- 检查 active task 是否冲突

如果项目已经进入 git / worktree 并行阶段，第二轮最值得补这 2 个：

### 6. `harness-prepare-worktree`

职责：

- 从 task 读取 `branchName` / `worktreePath` / `baseRef`
- 检查 worktree 是否已存在
- 必要时创建 worktree
- 给 worker 一个稳定执行目录

### 7. `harness-diff-summary`

职责：

- 从 task 读取 `diffBase` / `branchName` / `worktreePath`
- 生成 `git diff --stat` 摘要
- 可选回写 `diffSummary`
- 给 audit / operator 一个快速证据视图

如果下游模型偏弱，再补这个：

### 8. `harness-render-prompt`

职责：

- 按 `role` / `workerMode` / `stage` 渲染 prompt
- 第一层只输出骨架
- 后续阶段再展开执行边界和恢复动作
- 避免把全量 prompt 一次性塞给弱模型

## 任务友好显示建议

如果用户说“我要看当前正在进行的任务”，默认使用：

- `title`: 短标题
- `summary`: 当前一句话说明
- `description`: 细节

CLI 面板优先显示：

```text
current task: T-042
title: Fix incremental sync delta detection
summary: 修复增量同步误判，确保只处理变化数据。
```

不要把长 `description` 直接塞进总览。

## 谱系校验最值得先做的规则

### 深度

- 默认最大深度：4
- 推荐常态深度：3

### 可执行性

- 非 leaf task 默认不可被 worker 直接执行
- orchestration task 可以执行，但其 `ownedPaths` 应主要指向 `.harness/**`

### 冲突

默认视为冲突的场景：

- active tasks 的 `ownedPaths` 直接重叠
- active task 与其 ancestor / descendant 同时写执行路径
- sibling tasks 命中同一 merge gate

### 局部重编排

- 默认只允许 `self / subtree / parent`
- 默认禁止全局 replan 作为第一反应

## request 推荐存放

如果项目有 unattended / daemon，建议显式放这些文件：

```text
.harness/replan-requests.json
.harness/stop-requests.json
```

这样守护进程和 operator 都能直接读取。

## 示例 bash wrapper

```bash
#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
python3 "$ROOT/.harness/scripts/tree_check.py" "$@"
```

## 最小可抄模板

如果用户不想从零写脚本，优先参考这些模板：

- `examples/harness-status.example.sh`
- `examples/harness-query.example.sh`
- `examples/harness-dashboard.example.sh`
- `examples/harness-install-tools.example.sh`
- `examples/harness-route-session.example.sh`
- `examples/harness-render-prompt.example.sh`
- `examples/harness-prepare-worktree.example.sh`
- `examples/harness-diff-summary.example.sh`
- `examples/status.example.py`
- `examples/query-harness.example.py`
- `examples/refresh-state.example.py`
- `examples/route-session.example.py`
- `examples/render-prompt.example.py`
- `examples/prepare-worktree.example.py`
- `examples/diff-summary.example.py`
- `examples/tooling-manifest.example.json`

建议第一轮直接复制后再按项目字段微调，不要重新发明 CLI 入口。

## 示例 Python 核心思路

`tree_check.py` 至少检查：

- depth
- cycle
- missing parent
- child back-reference mismatch
- non-leaf active worker task
- active ownedPaths overlap

`claim_next.py` 至少判断：

- 是否 leaf
- dependsOn 是否满足
- claim 是否空闲
- 是否与 active conflict
- 是否命中 stop / replan request

## 不推荐的做法

- 只做 bash，不做结构化校验
- 只做 Python，不给 operator 稳定命令入口
- 把 orchestrator 逻辑全部写死在守护 prompt 里
- 让 worker 直接手改全局编排文件而没有 request / stop 协议
