# OpenClaw Dispatch

在以下情况读取本文件：

- 你要把 `.harness/` 接到 OpenClaw 调度链路
- 你要把任务投递到 `tmux` worker node
- 你要生成 worker / orchestrator 注入模板

默认流程：

- `cron` = 调度触发层
- `tmux` = worker node 层
- `gpt-5.4` = 编排 fallback / 语义性 session 判断 / audit prompt 精化层
- `gpt-5.3-codex` = worker 执行层
- `codex exec` / `codex exec resume` = 单次执行层
- `git branch + git worktree` = 代码隔离层
- `.harness/*` = 状态层

如果用户偏好 bash-first 管理，再补一层：

- `bin/harness-*` = operator control surface

## 核心约束

1. `tmux` 不是任务本身，而是可复用 worker node
2. 主会话 / orchestrator 先写 `.harness/`，再派发命令
3. `claim.nodeId` 与 `dispatch.targetSelector` 必须能对应到 node
4. worker 不是自己凭空知道该干什么，它只承接已声明好的执行
5. 是否 `resume` 旧 session，优先由程序 gate 根据任务关系和 session 记录做规则内判断，不交给 `gpt-5.3-codex` 自己猜
6. pre-worker routing 默认先跑程序 gate；只有 gate 判定 `needsOrchestrator=true` 时，才把 routing fallback 交给 `gpt-5.4`
7. 代码型 worker 默认优先在独立 worktree 中执行，不直接在共享仓库根目录落改动

## dispatch 最小契约

`dispatch` 至少表达：

- `runner`
- `targetKind`
- `targetSelector`
- `entryRole`
- `taskContextId`
- `logPath`
- `heartbeatPath`
- `maxParallelism`
- `cooldownSeconds`

推荐映射：

- `runner = "codex exec"`
- `targetKind = "worker-node"`
- `targetSelector = "tmux:<node-pattern>"`
- `routingModel = "gpt-5.4"`
- `executionModel = "gpt-5.3-codex"`
- `orchestrationSessionId = "<gpt-5.4-session-id>"`
- `commandProfile.standard = "codex exec --yolo"`
- `commandProfile.localCompat = "codex --yolo exec"`
- `worktreePath = "<repo>/.worktrees/<task-or-branch>"`
- `diffBase = "<stable-base-ref>"`

## git / diff / worktree 节奏

推荐默认节奏：

1. `orchestrator` 先决定 task 是否需要独立 worktree
2. 如果需要，先创建 branch 和 worktree，再写 claim / dispatch
3. `worker` 在自己的 worktree 内执行
4. `worker` 回写 `diffSummary`
5. `audit` 和 `merge gate` 优先看 `git diff --stat <diffBase>...HEAD`
6. `orchestrator` 串行 merge 或回收 worktree

推荐约束：

- 纯 `.harness` 控制面任务可以不单独开 worktree
- 代码型任务、mergeRequired 任务、共享路径高冲突任务，优先单独开 worktree
- 同一个 worktree 默认只承载一个活跃代码 task
- 同一个 branch 默认不要被多个活跃 worker 共同推进
- `diffBase` 优先用稳定 ref，例如 `baseRef` 或 integration branch，而不是临时工作区 HEAD

注意：

- `--yolo` 作为默认写法
- 长期复用模板统一写 `--yolo`

## 典型节奏

默认以 `orchestrator + worker` 为主。
如果项目需要结构性复核或结果验收，再补一条 `audit worker` lane，但仍算 worker，不单独长出新角色。
只有在 `spec.json` / `context-map.json` 真的需要重写时，才额外拉出 `planner`。

### planner heartbeat

- 先快速产出 draft orchestration
- 检查 spec 是否过期
- 检查 blocked / conflict / replan
- 用 `gpt-5.4` 判断新任务是 `fresh` 还是 `resume`
- 必要时刷新 `spec.json`、`work-items.json`

### orchestrator heartbeat

- 把 draft orchestration 推进到 refinement
- 回收过期 claim
- 展开可领取 task
- 维护持续的 `orchestrationSessionId`
- 创建和回收 worker branch / worktree
- 记录 worker 完成后可复用的 session_id
- 更新 dispatch / handoff / lineage
- 给下一轮 worker 生成稳定派工信息

### pre-worker routing

- 先跑 `harness-route-session`
- 程序 gate 先判断当前 task 是否可 claim，以及当前 task 用 `fresh` 还是 `resume`
- 如果任务存在多个依赖 session，先生成 `candidateResumeSessionIds`
- 如果只有一个安全候选且无严重冲突，直接写入 `preferredResumeSessionId`
- 写入 `sessionFamilyId`、`cacheAffinityKey`、`routingReason`
- 如果 gate 输出 `needsOrchestrator=true`，再让 `gpt-5.4` 在完整编排上下文里完成 fallback 判断
- 只有 routing 落盘后，才允许派发 `gpt-5.3-codex`

### worker run

- 先确认 pre-worker routing 已完成
- 找一个可执行 `T-*`
- 只认领 `execution-ready` 的 leaf task
- 读取 `preferredResumeSessionId` / `resumeStrategy`
- claim 时把实际绑定的 `boundSessionId` 写回 task
- 在 `worktreePath` 中执行
- 按 `ownedPaths` 写代码
- 用 `git diff --stat <diffBase>...HEAD` 或等价命令回写 `diffSummary`
- 跑验证
- 回写状态
- 如失败或报错，也要回写 `lastKnownSessionId` 和失败原因，供后续回调继续执行
- 退出

### audit worker run

- 先跑程序 gate；只有 gate 输出 `needsOrchestrator=true` 时才由 `gpt-5.4` 对当前 audit task 做 routing fallback 和 prompt 精化
- 再派发给 `gpt-5.3-codex`
- 先确认当前 audit task 已写明 `reviewOfTaskIds` 或 `auditScope`
- 只读取审计范围内的 task、handoff、lineage、verification 结果与相关代码证据
- 优先结合 `git diff --stat <diffBase>...<branchName>` 采样证据
- 不直接做业务实现
- 生成或刷新 `.harness/audit-report.md`
- 回写当前 audit task 的 `auditVerdict`
- 如果发现问题，优先写 `replan-requests.json` / `stop-requests.json`
- 把下一步交回 `orchestrator`

### daemon loop

第一版推荐把所有上游入口统一收束到同一个 runner：

- `claude / shell / codex / openclaw-cron`
- `-> .harness/bin/harness-runner tick`
- `-> tmux`
- `-> codex --yolo exec`

runner 的职责：

- 读取 `.harness/task-pool.json`
- 读取 `.harness/session-registry.json`
- 读取 `.harness/state/runner-state.json` / `runner-heartbeats.json`
- 检查 tmux session 是否仍活跃
- 决定当前是继续 attach、resume 还是 fresh exec
- 把执行结果、活跃 run、recoverable run 写回 hot state

OpenClaw-cron 在第一版里只做触发，不自行管理恢复策略；它调用同一个 `harness-runner tick` 即可。

## bash-first 推荐组件

如果用户希望“用 bash 就能管理”，优先把这几个动作封成脚本，而不是把逻辑散在 prompt 里：

- `harness-status`
- `harness-watch`
- `harness-query`
- `harness-dashboard`
- `harness-runner`
- `harness-metrics`
- `harness-render-prompt`
- `harness-prepare-worktree`
- `harness-diff-summary`
- `harness-claim-next`
- `harness-request-replan`
- `harness-request-stop`
- `harness-checkpoint`
- `harness-resolve-orchestrator`
- `harness-route-session`
- `harness-run-audit`

推荐职责：

- `harness-status`
  - 输出当前 focus、active task、active summary、blocker、risk
- `harness-query`
  - 以 JSON 输出 `overview / progress / current / blueprint / task / feedback`
  - 优先读 `.harness/state/*.json`
  - 给守护进程、面板、其他工具直接取状态
- `harness-dashboard`
  - 聚合 `overview / current / progress / blueprint / task`
  - 给人类 operator 一个直接可用的 CLI 面板
- `harness-runner`
  - 统一执行入口：检查 active run、recoverable run、dispatchable task
  - 把所有上游触发器收束到 `tmux -> codex --yolo exec`
  - 先跑程序 pre-worker gate，再决定 resume / fresh / fallback
- `harness-watch`
  - 自动刷新 status
- `harness-metrics`
  - 输出人类可读 + JSON 可读进度
- `harness-render-prompt`
  - 按 `start / execute / recover / audit` 分层渲染 prompt
  - 让弱模型先看最小骨架，再按需展开
- `harness-prepare-worktree`
  - 按 task 准备 branch / worktree
  - 给 worker 一个稳定执行目录
- `harness-diff-summary`
  - 基于 `diffBase...HEAD` 生成摘要
  - 给 audit / merge gate 复核
- `harness-claim-next`
  - 返回下一个安全可执行 leaf task
- `harness-request-replan`
  - 生成局部 replan request
- `harness-request-stop`
  - 对特定 task 发 `graceful` / `immediate` stop
- `harness-checkpoint`
  - 写当前进度与恢复点
- `harness-resolve-orchestrator`
  - 当 request 出现且 worker 槽位空出来时，生成或认领 orchestrator task
- `harness-route-session`
  - 先做程序化 pre-worker gate
  - 输出 `claimable / blocked / orchestrator_review`
  - 输出建议的 `resumeStrategy`、`preferredResumeSessionId`、`promptStages`
  - 只有在 `needsOrchestrator=true` 时才回退到 `gpt-5.4`
- `harness-run-audit`
  - 认领下一个可执行 audit task
  - 先调 `gpt-5.4` 生成审计 prompt
  - 再调 `gpt-5.3-codex` 执行审计
  - 回写 `audit-report.md` 与 `auditVerdict`

## 派发前 gate

DO NOT 在以下动作之前投递命令：

- 程序 pre-worker gate 已完成
- claim 已写入
- dispatch 已写入
- handoff 已写入
- `lineage.jsonl` 中已有对应调度事件

如果目标是 audit task，额外要求：

- `reviewOfTaskIds` 或 `auditScope` 已写入
- `audit-report.md` 路径已存在或已声明将由当前 task 创建

## 注入模板

### Worker

```text
你是 `worker`。
当前模型是 `gpt-5.3-codex`。
你不是 `planner`。
你不是 `orchestrator`。
不要自行改编排。
不要自行改 session routing。
只执行任务 `<TASK_ID>`。
只修改 `<TASK_ID>` 的 ownedPaths。

按下面顺序执行：
1. 读取 `.harness/progress.md`、`.harness/task-pool.json`、`.harness/session-registry.json`。
2. 找到 `<TASK_ID>`。
3. 如果存在 `.harness/state/feedback-summary.json`，只读取当前 task 最近 3 条高严重度失败。
4. 读取 `resumeStrategy`、`preferredResumeSessionId`、`candidateResumeSessionIds`、`worktreePath`、`diffBase`。
5. 写 claim，claim 中写入当前 nodeId=`<NODE_ID>`。
6. 把实际使用的 session 写入 `claim.boundSessionId`。
7. 在 `worktreePath` 中实现。
8. 先写 `diffSummary`，再验证。
9. 回写 `.harness/task-pool.json`、`.harness/progress.md`、`.harness/lineage.jsonl`、`.harness/session-registry.json`。

如果依赖未满足、路径冲突、需要 rollback、需要 stop 其他 task、或路径边界不清：
1. 写 request。
2. 回写 `lastKnownSessionId` 和失败原因。
3. 把 task 置为 `pause_requested` 或 `finishing_then_pause`。
4. 停止继续扩写。
```

### Audit Worker

```text
你是 `worker`。
当前模型是 `gpt-5.3-codex`。
你的 `workerMode = audit`。
你不是 `planner`。
你不是 `orchestrator`。

你只负责：
1. 复核 `<TASK_ID>` 指向的结果或子树
2. 采样证据
3. 写 `.harness/audit-report.md`
4. 写当前 audit task 的 `auditVerdict`

按下面顺序执行：
1. 读取 `.harness/progress.md`、`.harness/task-pool.json`、`.harness/lineage.jsonl`、`.harness/audit-report.md`。
2. 找到 `<TASK_ID>`。
3. 读取 `reviewOfTaskIds`、`auditScope`、`verificationRuleIds`、`handoff`、`branchName`、`diffBase`。
4. 只做证据采样和结论整理，不做业务实现。
5. 写 `audit-report.md`。
6. 把结论写回当前 task 的 `auditVerdict`。
7. 如发现问题，写 request，并把下一步交回 `orchestrator`。

禁止：
- 不要直接修改业务源码
- 不要直接改 `spec.json` / `work-items.json`
- 不要在没有证据时给出 `pass`
```

### Orchestrator

```text
你是 `orchestrator`。
当前模型是 `gpt-5.4`。
你不直接做 worker 实现。

你只负责：
1. 维护 `orchestrationSessionId`
2. 只在程序 gate 判定 `needsOrchestrator=true` 时做 routing fallback
3. 生成给 `gpt-5.3-codex` 的 worker prompt

先读取 `.harness/progress.md`、`.harness/work-items.json`、`.harness/task-pool.json`、`.harness/spec.json`、`.harness/standards.md`、`.harness/session-registry.json`、`.harness/state/feedback-summary.json`（如果存在）。

对每个待派发 task，至少写回：
- `orchestrationSessionId`
- `resumeStrategy`
- `preferredResumeSessionId`
- `candidateResumeSessionIds`
- `sessionFamilyId`
- `cacheAffinityKey`
- `routingReason`

规则：
- 程序 gate 已先做一轮 claimable / conflict / feedback 检查
- 同 parent、同冲突域、同角色、直接后续 => 优先 `resume`
- 跨 parent、角色切换、replan 后、旧 session 污染重 => 优先 `fresh`
- 不允许多个 worker 同时 `resume` 同一个 active session
- 最近失败窗口命中 `illegal_action` / `path_conflict` / `session_conflict` 时，优先 review / replan，不直接续跑

没写完 routing 字段前，不要放行 worker。
```

### 主会话派发

```text
目标 node: <NODE_ID>
目标任务: <TASK_ID>
编排模型: gpt-5.4
执行模型: gpt-5.3-codex
编排主线 session: <ORCHESTRATION_SESSION_ID>
pre-worker gate: 先跑 `harness-route-session <TASK_ID> .`
如果 gate 输出 `needsOrchestrator=true`:
  再 `codex exec resume <ORCHESTRATION_SESSION_ID> -m gpt-5.4 ...`
如果 `resumeStrategy=fresh`:
  标准启动方式: codex exec --yolo -m gpt-5.3-codex "<WORKER_PROMPT>"
  本机兼容写法: codex --yolo exec -m gpt-5.3-codex "<WORKER_PROMPT>"
如果 `resumeStrategy=resume`:
  标准启动方式: codex exec resume <SESSION_ID> --yolo -m gpt-5.3-codex "<WORKER_PROMPT>"
  本机兼容写法: codex exec resume <SESSION_ID> --yolo -m gpt-5.3-codex "<WORKER_PROMPT>"
要求：先写 claim / dispatch / lineage，再投递命令。
```

如果目标任务 `kind=audit`：

- 先跑程序 gate；只有 gate 输出 `needsOrchestrator=true` 时，才 `codex exec resume <ORCHESTRATION_SESSION_ID> -m gpt-5.4 ...` 生成精确 audit worker prompt
- 再用 `gpt-5.3-codex` 执行
- `entryRole = worker`
- `workerMode = audit`
- 默认优先 `fresh`，除非是同一 `auditScope` 的连续复核
